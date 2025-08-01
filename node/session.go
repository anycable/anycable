package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/logger"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/utils"
	"github.com/anycable/anycable-go/ws"
)

// Executor handles incoming commands (messages)
//
//go:generate mockery --name Executor --output "../node_mocks" --outpkg node_mocks
type Executor interface {
	HandleCommand(*Session, *common.Message) error
	Disconnect(*Session) error
}

type timerEvent uint8

const (
	timerEventHandshake timerEvent = 1
	timerEventPresence  timerEvent = 2
	timerEventExpire    timerEvent = 3
	timerEventPing      timerEvent = 4
	timerEventPong      timerEvent = 5
)

type SessionTimers struct {
	timer *time.Timer

	pingDeadline      int64
	handshakeDeadline int64
	pongDeadline      int64
	resumeDeadline    int64
	presenceDeadline  int64

	nextEvent    timerEvent
	timerHandler func(timerEvent)

	mu sync.Mutex
}

func (st *SessionTimers) Start(handler func(timerEvent)) {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.timerHandler = handler
	st.schedule()
}

func (st *SessionTimers) Stop() {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.timer != nil {
		st.timer.Stop()
	}
}

func (st *SessionTimers) Schedule() {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.schedule()
}

func (st *SessionTimers) schedule() {
	if st.timer != nil {
		st.timer.Stop()
	}

	var ev timerEvent
	var delay int64

	if st.pingDeadline > 0 {
		ev = timerEventPing
		delay = st.pingDeadline
	}

	if st.handshakeDeadline > 0 && (delay == 0 || st.handshakeDeadline < delay) {
		ev = timerEventHandshake
		delay = st.handshakeDeadline
	}

	if st.pongDeadline > 0 && (delay == 0 || st.pongDeadline < delay) {
		ev = timerEventPong
		delay = st.pongDeadline
	}

	if st.resumeDeadline > 0 && (delay == 0 || st.resumeDeadline < delay) {
		ev = timerEventExpire
		delay = st.resumeDeadline
	}

	if st.presenceDeadline > 0 && (delay == 0 || st.presenceDeadline < delay) {
		ev = timerEventPresence
		delay = st.presenceDeadline
	}

	if ev > 0 {
		st.nextEvent = ev
		after := time.Duration(delay-time.Now().UnixNano()) * time.Nanosecond
		if st.timer != nil {
			st.timer.Reset(after)
		} else {
			st.timer = time.AfterFunc(after, st.tick)
		}
	}
}

func (st *SessionTimers) tick() {
	st.mu.Lock()
	op := st.nextEvent
	st.mu.Unlock()

	st.timerHandler(op)
}

// Session represents active client
type Session struct {
	conn          Connection
	uid           string
	encoder       encoders.Encoder
	executor      Executor
	broker        broker.Broker
	metrics       metrics.Instrumenter
	env           *common.SessionEnv
	subscriptions *SubscriptionState
	closed        bool

	// Defines if we should perform Disconnect RPC for this session
	disconnectInterest bool

	// Main mutex (for important session updates)
	mu sync.Mutex
	// Mutex for protocol-related state (env, subscriptions)
	smu sync.Mutex
	// Mutex for writes
	wmu sync.Mutex

	writeQueue     *utils.Queue[*ws.SentFrame]
	writeTimeout   time.Duration
	maxPendingSize uint64

	timers *SessionTimers

	pingInterval           time.Duration
	pingTimestampPrecision string

	pongTimeout      time.Duration
	presenceInterval time.Duration
	resumeInterval   time.Duration

	prevSid string

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
		s.timers.handshakeDeadline = deadline.UnixNano()
	}
}

// WithMetrics allows to set a custom metrics instrumenter for a session
func WithMetrics(m metrics.Instrumenter) SessionOption {
	return func(s *Session) {
		s.metrics = m
	}
}

// WithBroker connects a session to a broker, so the session can send keepalive signals for
// cache and presence
func WithKeepaliveIntervals(sessionsTTL time.Duration, presenceTTL time.Duration) SessionOption {
	return func(s *Session) {
		s.resumeInterval = sessionsTTL
		s.presenceInterval = presenceTTL
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

// WithWriteTimeout allows to set a custom write timeout for a session
func WithWriteTimeout(timeout time.Duration) SessionOption {
	return func(s *Session) {
		s.writeTimeout = timeout
	}
}

// WithMaxPendingSize allows to set a custom max pending size for a session
func WithMaxPendingSize(size uint64) SessionOption {
	return func(s *Session) {
		s.maxPendingSize = size
	}
}

// BuildSession builds a new Session struct with the required defaults
func BuildSession(conn Connection, env *common.SessionEnv) *Session {
	return &Session{
		conn:          conn,
		metrics:       metrics.NoopMetrics{},
		env:           env,
		subscriptions: NewSubscriptionState(),
		writeQueue:    utils.NewQueue[*ws.SentFrame](256),
		writeTimeout:  2 * time.Second,
		closed:        false,
		Connected:     false,
		timers:        &SessionTimers{},
		// Use JSON by default
		encoder: encoders.JSON{},
	}
}

// NewSession build a new Session struct from ws connetion and http request
func NewSession(node *Node, conn Connection, url string, headers *map[string]string, uid string, opts ...SessionOption) *Session {
	session := BuildSession(conn, common.NewSessionEnv(url, headers))
	session.metrics = node.metrics
	session.executor = node
	session.broker = node.broker
	session.pingInterval = time.Duration(node.config.PingInterval) * time.Second
	session.pingTimestampPrecision = node.config.PingTimestampPrecision
	session.uid = uid

	ctx := node.log.With("sid", session.uid)

	session.Log = ctx

	for _, opt := range opts {
		opt(session)
	}

	go session.SendMessages()

	session.startTimers()

	return session
}

func (s *Session) startTimers() {
	s.scheduleInitialPing()
	s.scheduleResumeability()
	s.schedulePresence()

	s.timers.Start(s.handleTimerEvent)
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
	return s.timers.handshakeDeadline == 0
}

func (s *Session) IsConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.Connected
}

func (s *Session) IsResumeable() bool {
	return s.resumeInterval > 0
}

func (s *Session) maybeDisconnectIdle() {
	// reset handshake deadline
	s.timers.mu.Lock()
	s.timers.handshakeDeadline = 0
	s.timers.mu.Unlock()

	s.mu.Lock()

	if s.Connected {
		s.mu.Unlock()
		s.timers.Schedule()
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

// SendMessages waits for incoming messages and send them to the client connection
func (s *Session) SendMessages() {
	for {
		if !s.writeQueue.Wait() {
			return
		}

		item, ok := s.writeQueue.Remove()
		if !ok {
			return
		}

		message := item.Data

		err := s.writeFrame(message)

		if message.FrameType == ws.CloseFrame {
			s.disconnectNow("Close frame sent", ws.CloseNormalClosure)
			return
		}

		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				s.Log.Debug("write timed out", "error", err, "msg", logger.CompactValue(message.Payload))
				s.metrics.CounterIncrement(metricsWriteTimeout)
			}
			s.metrics.CounterIncrement(metricsFailedSent)
			s.disconnectNow("Write Failed", ws.CloseAbnormalClosure)
			return
		}

		s.metrics.CounterIncrement(metricsSentMsg)
	}
}

// ReadMessage reads messages from ws connection and send them to node
func (s *Session) ReadMessage(message []byte) error {
	s.metrics.CounterAdd(metricsDataReceived, uint64(len(message)))

	command, err := s.decodeMessage(message)

	if err != nil {
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
		s.Log.Warn("failed to encode message", "data", msg, "type", msg.GetType(), "error", err)
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

	s.disconnectFromNode()
	s.writeFrame(&ws.SentFrame{ // nolint:errcheck
		FrameType:   ws.CloseFrame,
		CloseReason: reason,
		CloseCode:   code,
	})

	if !s.writeQueue.Closed() {
		s.writeQueue.Close()
	}

	s.close()
}

func (s *Session) close() {
	s.mu.Lock()

	if s.closed {
		s.mu.Unlock()
		return
	}

	s.closed = true
	s.mu.Unlock()

	s.timers.Stop()
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
	if s.writeQueue.Closed() {
		return
	}

	pendingTotal := s.writeQueue.Size()
	s.mu.Lock()
	maxPending := s.maxPendingSize
	s.mu.Unlock()

	if maxPending > 0 && pendingTotal >= maxPending {
		s.writeQueue.Clear()
		s.mu.Lock()
		s.maxPendingSize = 0
		s.mu.Unlock()

		s.Log.Debug("slow client detected, disconnecting", "pending", pendingTotal, "max", s.maxPendingSize)

		s.metrics.CounterIncrement(metricsDisconnectedSlowClients)
		s.DisconnectWithMessage(common.NewDisconnectMessage(common.SLOW_CLIENT_REASON, true), common.SLOW_CLIENT_REASON)
		return
	}

	size := len(message.Payload)

	item := utils.Item[*ws.SentFrame]{Data: message, Size: uint64(size)}

	if !s.writeQueue.Add(item) {
		defer s.Disconnect("Write failed", ws.CloseAbnormalClosure)
	}
}

func (s *Session) writeFrame(message *ws.SentFrame) error {
	return s.writeFrameWithDeadline(message, time.Now().Add(s.writeTimeout))
}

func (s *Session) writeFrameWithDeadline(message *ws.SentFrame, deadline time.Time) error {
	switch message.FrameType {
	case ws.TextFrame:
		s.wmu.Lock()
		defer s.wmu.Unlock()

		err := s.conn.Write(message.Payload, deadline)
		if err == nil {
			s.metrics.CounterAdd(metricsDataSent, uint64(len(message.Payload)))
		}
		return err
	case ws.BinaryFrame:
		s.wmu.Lock()
		defer s.wmu.Unlock()

		err := s.conn.WriteBinary(message.Payload, deadline)
		if err == nil {
			s.metrics.CounterAdd(metricsDataSent, uint64(len(message.Payload)))
		}
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

	s.schedulePing()
}

func (s *Session) scheduleInitialPing() {
	if s.pingInterval <= 0 {
		return
	}

	s.timers.mu.Lock()
	defer s.timers.mu.Unlock()

	// Calculate the minimum and maximum durations
	minDuration := s.pingInterval / 2
	maxDuration := s.pingInterval * 3 / 2

	initialInterval := time.Duration(rand.Int63n(int64(maxDuration-minDuration))) + minDuration // nolint:gosec

	s.timers.pingDeadline = time.Now().Add(initialInterval).UnixNano()

	if s.pongTimeout > 0 {
		s.timers.pongDeadline = time.Now().Add(s.pongTimeout + initialInterval).UnixNano()
	}
}

func (s *Session) schedulePing() {
	s.timers.mu.Lock()
	defer s.timers.mu.Unlock()

	s.timers.pingDeadline = time.Now().Add(s.pingInterval).UnixNano()

	if s.pongTimeout > 0 && s.timers.pongDeadline == 0 {
		s.timers.pongDeadline = time.Now().Add(s.pongTimeout).UnixNano()
	}

	s.timers.schedule()
}

func (s *Session) scheduleResumeability() {
	s.timers.mu.Lock()
	defer s.timers.mu.Unlock()

	if s.resumeInterval > 0 {
		s.timers.resumeDeadline = time.Now().Add(s.resumeInterval).UnixNano()
	}
}

func (s *Session) schedulePresence() {
	s.timers.mu.Lock()
	defer s.timers.mu.Unlock()

	if s.presenceInterval > 0 {
		s.timers.presenceDeadline = time.Now().Add(s.presenceInterval).UnixNano()
	}
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

// keepalive is called when we receive a message from the client
func (s *Session) keepalive() {
	s.timers.mu.Lock()
	defer s.timers.mu.Unlock()

	needReschedule := false

	if s.timers.pongDeadline > 0 {
		s.timers.pongDeadline = 0
		needReschedule = true
	}

	if needReschedule {
		s.timers.schedule()
	}
}

func (s *Session) resetPong() {
	s.timers.mu.Lock()
	defer s.timers.mu.Unlock()

	if s.timers.pongDeadline <= 0 {
		return
	}

	s.timers.pongDeadline += s.pongTimeout.Nanoseconds()
	s.timers.schedule()
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

func (s *Session) handleClientPing(msg *common.Message) error {
	s.Log.Debug("ping received")
	s.Send(&common.Reply{Type: common.PongType})
	return nil
}

func (s *Session) handleTimerEvent(ev timerEvent) {
	if s.IsClosed() {
		return
	}

	switch ev {
	case timerEventHandshake:
		s.maybeDisconnectIdle()
	case timerEventPresence:
		s.broker.TouchPresence(s.GetID()) // nolint:errcheck
		s.schedulePresence()
		s.timers.Schedule()
	case timerEventExpire:
		s.broker.TouchSession(s.GetID()) // nolint:errcheck
		s.scheduleResumeability()
		s.timers.Schedule()
	case timerEventPing:
		s.sendPing()
	case timerEventPong:
		s.handleNoPong()
	}
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
