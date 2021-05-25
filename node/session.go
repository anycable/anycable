package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/logger"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/netpoll"
	"github.com/anycable/anycable-go/utils"
	"github.com/anycable/anycable-go/ws"

	gobwas "github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

const (
	writeWait = 10 * time.Second
)

// Executor handles incoming commands (messages)
type Executor interface {
	HandleCommand(*Session, *common.Message) error
	Disconnect(*Session) error
}

// Session represents active client
type Session struct {
	conn          Connection
	uid           string
	encoder       encoders.Encoder
	executor      Executor
	metrics       metrics.Instrumenter
	env           *common.SessionEnv
	subscriptions *SubscriptionState
	closed        bool

	// Defines if we should perform Disconnect RPC for this session
	disconnectInterest bool

	// Main mutex (for read/write and important session updates)
	mu sync.Mutex
	// Mutex for protocol-related state (env, subscriptions)
	smu sync.Mutex

	readPool  *utils.GoPool
	writePool *utils.GoPool

	// Mutex for activating SendMessages worker
	wmu             sync.Mutex
	workerRunning   bool
	workerScheduled bool

	sendCh          chan *ws.SentFrame
	sendChannelOpen bool

	pingTimer    *time.Timer
	pingInterval time.Duration

	pingTimestampPrecision string

	handshakeDeadline time.Time

	pongTimeout time.Duration
	pongTimer   *time.Timer

	resumable bool
	prevSid   string

	Connected bool
	// Could be used to store arbitrary data within a session
	InternalState map[string]interface{}
	Log           *slog.Logger
}

type SessionOption = func(*Session)

// WithPingInterval allows to set a custom ping interval for a session
// or disable pings at all (by passing 0)
func WithPingInterval(interval time.Duration) SessionOption {
	return func(s *Session) {
		s.pingInterval = interval
	}
}

// WithPingPrecision allows to configure precision for timestamps attached to pings
func WithPingPrecision(val string) SessionOption {
	return func(s *Session) {
		s.pingTimestampPrecision = val
	}
}

// WithEncoder allows to set a custom encoder for a session
func WithEncoder(enc encoders.Encoder) SessionOption {
	return func(s *Session) {
		s.encoder = enc
	}
}

// WithExecutor allows to set a custom executor for a session
func WithExecutor(ex Executor) SessionOption {
	return func(s *Session) {
		s.executor = ex
	}
}

// WithHandshakeMessageDeadline allows to set a custom deadline for handshake messages.
// This option also indicates that we MUST NOT perform Authenticate on connect.
func WithHandshakeMessageDeadline(deadline time.Time) SessionOption {
	return func(s *Session) {
		s.handshakeDeadline = deadline
	}
}

// WithMetrics allows to set a custom metrics instrumenter for a session
func WithMetrics(m metrics.Instrumenter) SessionOption {
	return func(s *Session) {
		s.metrics = m
	}
}

// WithResumable allows marking session as resumable (so we store its state in cache)
func WithResumable(val bool) SessionOption {
	return func(s *Session) {
		s.resumable = val
	}
}

// WithPrevSID allows providing the previous session ID to restore from
func WithPrevSID(sid string) SessionOption {
	return func(s *Session) {
		s.prevSid = sid
	}
}

// WithPongTimeout allows to set a custom pong timeout for a session
func WithPongTimeout(timeout time.Duration) SessionOption {
	return func(s *Session) {
		s.pongTimeout = timeout
	}
}

// NewSession build a new Session struct from ws connetion and http request
func NewSession(node *Node, conn Connection, url string, headers *map[string]string, uid string, opts ...SessionOption) *Session {
	session := &Session{
		conn:                   conn,
		metrics:                node.metrics,
		env:                    common.NewSessionEnv(url, headers),
		subscriptions:          NewSubscriptionState(),
		sendCh:                 make(chan *ws.SentFrame, 256),
		sendChannelOpen:        true,
		closed:                 false,
		Connected:              false,
		pingInterval:           time.Duration(node.config.PingInterval) * time.Second,
		pingTimestampPrecision: node.config.PingTimestampPrecision,
		// Use JSON by default
		encoder: encoders.JSON{},
		// Use Action Cable executor by default (implemented by node)
		executor:  node,
		readPool:  node.readPool,
		writePool: node.writePool,
	}

	session.uid = uid

	ctx := node.log.With("sid", session.uid)

	session.Log = ctx

	for _, opt := range opts {
		opt(session)
	}

	if session.pingInterval > 0 {
		session.startPing()
	}

	if !session.handshakeDeadline.IsZero() {
		val := time.Until(session.handshakeDeadline)
		time.AfterFunc(val, session.maybeDisconnectIdle)
	}

	return session
}

func (s *Session) GetEnv() *common.SessionEnv {
	return s.env
}

func (s *Session) SetEnv(env *common.SessionEnv) {
	s.env = env
}

func (s *Session) UnderlyingConn() Connection {
	return s.conn
}

func (s *Session) AuthenticateOnConnect() bool {
	return s.handshakeDeadline.IsZero()
}

func (s *Session) IsConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.Connected
}

func (s *Session) IsResumeable() bool {
	return s.resumable
}

func (s *Session) maybeDisconnectIdle() {
	s.mu.Lock()

	if s.Connected {
		s.mu.Unlock()
		return
	}

	s.mu.Unlock()

	s.Log.Warn("disconnecting idle session")

	s.Send(common.NewDisconnectMessage(common.IDLE_TIMEOUT_REASON, false))
	s.Disconnect("Idle Timeout", ws.CloseNormalClosure)
}

func (s *Session) GetID() string {
	return s.uid
}

func (s *Session) SetID(id string) {
	s.uid = id
}

func (s *Session) GetIdentifiers() string {
	return s.env.Identifiers
}

func (s *Session) SetIdentifiers(ids string) {
	s.env.Identifiers = ids
}

// Merge connection and channel states into current env.
// This method locks the state for writing (so, goroutine-safe)
func (s *Session) MergeEnv(env *common.SessionEnv) {
	s.smu.Lock()
	defer s.smu.Unlock()

	if env.ConnectionState != nil {
		s.env.MergeConnectionState(env.ConnectionState)
	}

	if env.ChannelStates != nil {
		states := *env.ChannelStates
		for id, state := range states { // #nosec
			s.env.MergeChannelState(id, &state)
		}
	}
}

// WriteInternalState
func (s *Session) WriteInternalState(key string, val interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.InternalState == nil {
		s.InternalState = make(map[string]interface{})
	}

	s.InternalState[key] = val
}

// ReadInternalState reads internal state value by key
func (s *Session) ReadInternalState(key string) (interface{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.InternalState == nil {
		return nil, false
	}

	val, ok := s.InternalState[key]

	return val, ok
}

func (s *Session) IsDisconnectable() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.disconnectInterest
}

func (s *Session) MarkDisconnectable(val bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.disconnectInterest = s.disconnectInterest || val
}

// Serve enters a loop to read incoming data
func (s *Session) Serve(callback func()) error {
	go func() {
		defer callback()

		for {
			if s.IsClosed() {
				return
			}

			message, err := s.conn.Read()

			if err != nil {
				if ws.IsCloseError(err) {
					s.Log.Debug("WebSocket closed", "error", err)
					s.disconnectNow("Read closed", ws.CloseNormalClosure)
				} else {
					s.Log.Debug("WebSocket close error", "error", err)
					s.disconnectNow("Read failed", ws.CloseAbnormalClosure)
				}
				return
			}

			err = s.ReadMessage(message)

			if err != nil {
				s.Log.Debug("WebSocket read failed", "error", err)
				return
			}
		}
	}()

	return nil
}

// ServeWithPoll register the connection within a netpoll and subscribes to Read/Close events
func (s *Session) ServeWithPoll(poller netpoll.Poller, callback func()) error {
	wsConn := s.conn.(*ws.Connection)

	fd := wsConn.Fd()

	// Connection has been already closed
	if fd < 0 {
		return nil
	}

	desc, err := netpoll.HandleReadOnce(fd)

	if err != nil {
		return err
	}

	s.Log.Debug("adding descriptor to poller")

	retried := false

ADD:
	err = poller.Start(desc, func(ev netpoll.Event) {
		if ev&(netpoll.EventReadHup|netpoll.EventHup) != 0 {
			poller.Stop(desc) // nolint:errcheck
			desc.Close()

			if ev&(netpoll.EventErr) != 0 {
				s.Log.Debug("descriptor closed with error")
				s.metrics.CounterIncrement(metricsAbnormalSocketClosure)
				s.cleanup()
			} else {
				s.Log.Debug("descriptor closed", "event", ev)
				s.disconnectNow("Closed", ws.CloseNormalClosure)
			}

			callback()
			return
		}

		s.metrics.GaugeIncrement(metricsReadPoolPendingNum)
		s.readPool.Schedule(func() {
			s.metrics.GaugeDecrement(metricsReadPoolPendingNum)

			wsconn := s.conn.Descriptor()
			message, op, rerr := wsutil.ReadClientData(wsconn)
			if op == gobwas.OpClose {
				s.Log.Debug("WebSocket closed", "error", rerr)
				s.disconnectNow("Read closed", ws.CloseNormalClosure)
				return
			}

			if rerr != nil {
				s.Log.Debug("WebSocket read failed", "error", rerr)
				poller.Stop(desc) // nolint:errcheck
				desc.Close()
				s.disconnectNow("Read error", ws.CloseAbnormalClosure)
				callback()
				return
			}

			if message != nil {
				s.ReadMessage(message) // nolint:errcheck
			}
			poller.Resume(desc) // nolint:errcheck
		})
	})

	// There could be a race condition when we haven't removed the closed
	// socket from the poller; in this case, we force-remove it and try adding
	// the descriptor again
	if err == netpoll.ErrRegistered && !retried {
		retried = true
		s.Log.Debug("retried to add fd")
		poller.Stop(desc) // nolint:errcheck
		goto ADD
	}

	return err
}

// SendMessages waits for incoming messages and send them to the client connection
func (s *Session) SendMessages() {
	for {
		select {
		case message, ok := <-s.sendCh:
			if !ok {
				return
			}

			err := s.writeFrame(message)

			if message.FrameType == ws.CloseFrame {
				s.disconnectNow("Close frame sent", ws.CloseNormalClosure)
				return
			}

			if err != nil {
				s.metrics.CounterIncrement(metricsFailedSent)
				s.disconnectNow("Write Failed", ws.CloseAbnormalClosure)
				return
			}

			s.metrics.CounterIncrement(metricsSentMsg)
		default:
			return
		}
	}
}

// ReadMessage reads messages from ws connection and send them to node
func (s *Session) ReadMessage(message []byte) error {
	s.metrics.CounterAdd(metricsDataReceived, uint64(len(message)))

	command, err := s.decodeMessage(message)

	if err != nil {
		s.Log.Debug("Websocket read failed", "error", err)
		s.metrics.CounterIncrement(metricsFailedCommandReceived)
		return err
	}

	if command == nil {
		return nil
	}

	s.metrics.CounterIncrement(metricsReceivedMsg)

	if err := s.executor.HandleCommand(s, command); err != nil {
		s.metrics.CounterIncrement(metricsFailedCommandReceived)
		s.Log.Warn("failed to handle incoming message", "data", logger.CompactValue(message), "error", err)
	}

	return nil
}

// Send schedules a data transmission
func (s *Session) Send(msg encoders.EncodedMessage) {
	if b, err := s.encodeMessage(msg); err == nil {
		if b != nil {
			s.sendFrame(b)
		}
	} else {
		s.Log.Warn("failed to encode message", "data", msg, "error", err)
	}
}

// SendJSONTransmission is used to propagate the direct transmission to the client
// (from RPC call result)
func (s *Session) SendJSONTransmission(msg string) {
	if b, err := s.encodeTransmission(msg); err == nil {
		if b != nil {
			s.sendFrame(b)
		}
	} else {
		s.Log.Warn("failed to encode transmission", "data", logger.CompactValue(msg), "error", err)
	}
}

// Disconnect schedules connection disconnect
func (s *Session) Disconnect(reason string, code int) {
	s.sendClose(reason, code)
	s.close()
	s.disconnectFromNode()
}

func (s *Session) DisconnectWithMessage(msg encoders.EncodedMessage, code string) {
	s.Send(msg)

	reason := ""
	wsCode := ws.CloseNormalClosure

	switch code {
	case common.SERVER_RESTART_REASON:
		reason = "Server restart"
		wsCode = ws.CloseGoingAway
	case common.REMOTE_DISCONNECT_REASON:
		reason = "Closed remotely"
	}

	s.Disconnect(reason, wsCode)
}

// String returns session string representation (for %v in Printf-like functions)
func (s *Session) String() string {
	return fmt.Sprintf("Session(%s)", s.uid)
}

type cacheEntry struct {
	Identifiers     string                       `json:"ids"`
	Subscriptions   map[string][]string          `json:"subs"`
	ConnectionState map[string]string            `json:"cstate"`
	ChannelsState   map[string]map[string]string `json:"istate"`
	Disconnectable  bool
}

func (s *Session) ToCacheEntry() ([]byte, error) {
	s.smu.Lock()
	defer s.smu.Unlock()

	entry := cacheEntry{
		Identifiers:     s.GetIdentifiers(),
		Subscriptions:   s.subscriptions.ToMap(),
		ConnectionState: *s.env.ConnectionState,
		ChannelsState:   *s.env.ChannelStates,
		Disconnectable:  s.disconnectInterest,
	}

	return json.Marshal(&entry)
}

func (s *Session) RestoreFromCache(cached []byte) error {
	var entry cacheEntry

	err := json.Unmarshal(cached, &entry)

	if err != nil {
		return err
	}

	s.smu.Lock()
	defer s.smu.Unlock()

	s.MarkDisconnectable(entry.Disconnectable)
	s.SetIdentifiers(entry.Identifiers)
	s.env.MergeConnectionState(&entry.ConnectionState)

	for k := range entry.ChannelsState {
		v := entry.ChannelsState[k]
		s.env.MergeChannelState(k, &v)
	}

	for k, v := range entry.Subscriptions {
		s.subscriptions.AddChannel(k)

		for _, stream := range v {
			s.subscriptions.AddChannelStream(k, stream)
		}
	}

	return nil
}

func (s *Session) PrevSid() string {
	return s.prevSid
}

func (s *Session) disconnectFromNode() {
	s.mu.Lock()
	if s.Connected {
		defer s.executor.Disconnect(s) // nolint:errcheck
	}
	s.Connected = false
	s.mu.Unlock()
}

func (s *Session) DisconnectNow(reason string, code int) {
	s.disconnectNow(reason, code)
}

func (s *Session) disconnectNow(reason string, code int) {
	s.mu.Lock()
	if !s.Connected {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	s.writeFrame(&ws.SentFrame{ // nolint:errcheck
		FrameType:   ws.CloseFrame,
		CloseReason: reason,
		CloseCode:   code,
	})

	s.cleanup()
}

func (s *Session) cleanup() {
	s.disconnectFromNode()

	s.mu.Lock()
	if s.sendChannelOpen {
		close(s.sendCh)
		s.sendChannelOpen = false
	}
	s.mu.Unlock()

	s.close()
}

func (s *Session) close() {
	s.mu.Lock()

	if s.closed {
		s.mu.Unlock()
		return
	}

	s.closed = true
	defer s.mu.Unlock()

	if s.pingTimer != nil {
		s.pingTimer.Stop()
	}

	if s.pongTimer != nil {
		s.pongTimer.Stop()
	}
}

func (s *Session) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.closed
}

func (s *Session) sendClose(reason string, code int) {
	s.sendFrame(&ws.SentFrame{
		FrameType:   ws.CloseFrame,
		CloseReason: reason,
		CloseCode:   code,
	})
}

func (s *Session) sendFrame(message *ws.SentFrame) {
	s.mu.Lock()

	if !s.sendChannelOpen {
		s.mu.Unlock()
		return
	}

	s.sendCh <- message
	s.mu.Unlock()

	s.ensureWorkerRunning()
}

func (s *Session) writeFrame(message *ws.SentFrame) error {
	return s.writeFrameWithDeadline(message, time.Now().Add(writeWait))
}

func (s *Session) writeFrameWithDeadline(message *ws.SentFrame, deadline time.Time) error {
	s.metrics.CounterAdd(metricsDataSent, uint64(len(message.Payload)))

	switch message.FrameType {
	case ws.TextFrame:
		s.mu.Lock()
		defer s.mu.Unlock()

		err := s.conn.Write(message.Payload, deadline)
		return err
	case ws.BinaryFrame:
		s.mu.Lock()
		defer s.mu.Unlock()

		err := s.conn.WriteBinary(message.Payload, deadline)

		return err
	case ws.CloseFrame:
		s.conn.Close(message.CloseCode, message.CloseReason)
		return errors.New("closed")
	default:
		s.Log.Error("unknown frame type", "msg", message)
		return errors.New("unknown frame type")
	}
}

func (s *Session) sendPing() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	deadline := time.Now().Add(s.pingInterval / 2)

	b, err := s.encodeMessage(newPingMessage(s.pingTimestampPrecision))

	if err != nil {
		s.Log.Error("failed to encode ping message", "error", err)
	} else if b != nil {
		err = s.writeFrameWithDeadline(b, deadline)
	}

	if err != nil {
		s.Disconnect("Ping failed", ws.CloseAbnormalClosure)
		return
	}

	s.addPing()
}

func (s *Session) startPing() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Calculate the minimum and maximum durations
	minDuration := s.pingInterval / 2
	maxDuration := s.pingInterval * 3 / 2

	initialInterval := time.Duration(rand.Int63n(int64(maxDuration-minDuration))) + minDuration // nolint:gosec

	s.pingTimer = time.AfterFunc(initialInterval, s.sendPing)

	if s.pongTimeout > 0 {
		s.pongTimer = time.AfterFunc(s.pongTimeout+initialInterval, s.handleNoPong)
	}
}

func (s *Session) addPing() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pingTimer = time.AfterFunc(s.pingInterval, s.sendPing)
}

func newPingMessage(format string) *common.PingMessage {
	var ts int64

	switch format {
	case "ns":
		ts = time.Now().UnixNano()
	case "ms":
		ts = time.Now().UnixNano() / int64(time.Millisecond)
	default:
		ts = time.Now().Unix()
	}

	return (&common.PingMessage{Type: "ping", Message: ts})
}

func (s *Session) handlePong(msg *common.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pongTimer == nil {
		s.Log.Debug("unexpected pong received")
		return
	}

	s.pongTimer.Reset(s.pongTimeout)
}

func (s *Session) handleNoPong() {
	s.mu.Lock()

	if !s.Connected {
		s.mu.Unlock()
		return
	}

	s.mu.Unlock()

	s.Log.Warn("disconnecting session due to no pongs")

	s.Send(common.NewDisconnectMessage(common.NO_PONG_REASON, true)) // nolint:errcheck
	s.Disconnect("No Pong", ws.CloseNormalClosure)
}

func (s *Session) encodeMessage(msg encoders.EncodedMessage) (*ws.SentFrame, error) {
	if cm, ok := msg.(*encoders.CachedEncodedMessage); ok {
		return cm.Fetch(
			s.encoder.ID(),
			func(m encoders.EncodedMessage) (*ws.SentFrame, error) {
				return s.encoder.Encode(m)
			})
	}

	return s.encoder.Encode(msg)
}

func (s *Session) encodeTransmission(msg string) (*ws.SentFrame, error) {
	return s.encoder.EncodeTransmission(msg)
}

func (s *Session) decodeMessage(raw []byte) (*common.Message, error) {
	return s.encoder.Decode(raw)
}

func (s *Session) ensureWorkerRunning() {
	s.wmu.Lock()

	if s.workerRunning || s.workerScheduled {
		s.wmu.Unlock()
		return
	}

	s.workerScheduled = true

	s.wmu.Unlock()

	s.metrics.GaugeIncrement(metricsWritePoolPendingNum)
	s.writePool.Schedule(func() {
		s.metrics.GaugeDecrement(metricsWritePoolPendingNum)

		s.wmu.Lock()
		if s.workerRunning {
			s.wmu.Unlock()
			return
		}

		s.workerScheduled = false
		s.workerRunning = true
		s.wmu.Unlock()

		s.SendMessages()

		s.wmu.Lock()
		s.workerRunning = false
		s.wmu.Unlock()
	})
}
