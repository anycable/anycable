package node

import (
	"encoding/json"
	"errors"
	"net/url"
	"sync"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/ws"
	"github.com/apex/log"
)

const (
	writeWait = 10 * time.Second

	prevSessionHeader = "X-ANYCABLE-RESTORE-SID"
	prevSessionParam  = "sid"
)

// Executor handles incoming commands (messages)
type Executor interface {
	HandleCommand(*Session, *common.Message) error
	Disconnect(*Session) error
}

type SubscriptionState struct {
	channels map[string]map[string]struct{}
	mu       sync.RWMutex
}

func NewSubscriptionState() *SubscriptionState {
	return &SubscriptionState{channels: make(map[string]map[string]struct{})}
}

func (st *SubscriptionState) HasChannel(id string) bool {
	st.mu.RLock()
	defer st.mu.RUnlock()

	_, ok := st.channels[id]
	return ok
}

func (st *SubscriptionState) AddChannel(id string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.channels[id] = make(map[string]struct{})
}

func (st *SubscriptionState) RemoveChannel(id string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	delete(st.channels, id)
}

func (st *SubscriptionState) Channels() []string {
	st.mu.RLock()
	defer st.mu.RUnlock()

	keys := make([]string, len(st.channels))
	i := 0

	for k := range st.channels {
		keys[i] = k
		i++
	}
	return keys
}

func (st *SubscriptionState) ToMap() map[string][]string {
	st.mu.RLock()
	defer st.mu.RUnlock()

	res := make(map[string][]string, len(st.channels))

	for k, v := range st.channels {
		streams := make([]string, len(v))

		i := 0
		for name := range v {
			streams[i] = name
			i++
		}

		res[k] = streams
	}

	return res
}

func (st *SubscriptionState) AddChannelStream(id string, stream string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if _, ok := st.channels[id]; ok {
		st.channels[id][stream] = struct{}{}
	}
}

func (st *SubscriptionState) RemoveChannelStream(id string, stream string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if _, ok := st.channels[id]; ok {
		delete(st.channels[id], stream)
	}
}

func (st *SubscriptionState) RemoveChannelStreams(id string) []string {
	st.mu.Lock()
	defer st.mu.Unlock()

	if streamNames, ok := st.channels[id]; ok {
		st.channels[id] = make(map[string]struct{})

		streams := make([]string, len(streamNames))

		i := 0
		for key := range streamNames {
			streams[i] = key
			i++
		}

		return streams
	}

	return nil
}

func (st *SubscriptionState) StreamsFor(id string) []string {
	st.mu.RLock()
	defer st.mu.RUnlock()

	if streamNames, ok := st.channels[id]; ok {
		streams := make([]string, len(streamNames))

		i := 0
		for key := range streamNames {
			streams[i] = key
			i++
		}

		return streams
	}

	return nil
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

	sendCh chan *ws.SentFrame

	pingTimer    *time.Timer
	pingInterval time.Duration

	pingTimestampPrecision string

	Connected bool
	// Could be used to store arbitrary data within a session
	InternalState map[string]interface{}
	Log           *log.Entry
}

// NewSession build a new Session struct from ws connetion and http request
func NewSession(node *Node, conn Connection, url string, headers *map[string]string, uid string) *Session {
	session := &Session{
		conn:                   conn,
		metrics:                node.metrics,
		env:                    common.NewSessionEnv(url, headers),
		subscriptions:          NewSubscriptionState(),
		sendCh:                 make(chan *ws.SentFrame, 256),
		closed:                 false,
		Connected:              false,
		pingInterval:           time.Duration(node.config.PingInterval) * time.Second,
		pingTimestampPrecision: node.config.PingTimestampPrecision,
		// Use JSON by default
		encoder: encoders.JSON{},
		// Use Action Cable executor by default (implemented by node)
		executor: node,
	}

	session.uid = uid

	ctx := node.log.WithFields(log.Fields{
		"sid": session.uid,
	})

	session.Log = ctx

	session.addPing()
	go session.SendMessages()

	return session
}

func (s *Session) SetEncoder(enc encoders.Encoder) {
	s.encoder = enc
}

func (s *Session) SetExecutor(ex Executor) {
	s.executor = ex
}

func (s *Session) SetMetrics(m metrics.Instrumenter) {
	s.metrics = m
}

func (s *Session) GetEnv() *common.SessionEnv {
	return s.env
}

func (s *Session) SetEnv(env *common.SessionEnv) {
	s.env = env
}

func (s *Session) SetIdleTimeout(val time.Duration) {
	time.AfterFunc(val, s.maybeDisconnectIdle)
}

func (s *Session) maybeDisconnectIdle() {
	s.mu.Lock()

	if s.Connected {
		s.mu.Unlock()
		return
	}

	s.mu.Unlock()

	s.Log.Warnf("Disconnecting idle session")

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
			message, err := s.conn.Read()

			if err != nil {
				if ws.IsCloseError(err) {
					s.Log.Debugf("Websocket closed: %v", err)
					s.disconnectNow("Read closed", ws.CloseNormalClosure)
				} else {
					s.Log.Debugf("Websocket close error: %v", err)
					s.disconnectNow("Read failed", ws.CloseAbnormalClosure)
				}
				return
			}

			err = s.ReadMessage(message)

			if err != nil {
				return
			}
		}
	}()

	return nil
}

// SendMessages waits for incoming messages and send them to the client connection
func (s *Session) SendMessages() {
	defer s.disconnectNow("Write Failed", ws.CloseAbnormalClosure)

	for message := range s.sendCh {
		err := s.writeFrame(message)

		if err != nil {
			s.metrics.CounterIncrement(metricsFailedSent)
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
		s.Log.Warnf("Failed to handle incoming message '%s' with error: %v", message, err)
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
		s.Log.Warnf("Failed to encode message %v. Error: %v", msg, err)
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
		s.Log.Warnf("Failed to encode transmission %v. Error: %v", msg, err)
	}
}

// Disconnect schedules connection disconnect
func (s *Session) Disconnect(reason string, code int) {
	s.disconnectFromNode()
	s.sendClose(reason, code)
	s.close()
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

func (s *Session) PrevSid() (psid string) {
	if s.env.Headers != nil {
		if v, ok := (*s.env.Headers)[prevSessionHeader]; ok {
			psid = v
			// This header is of one-time use,
			// no need to leak it to the RPC app
			delete(*s.env.Headers, prevSessionHeader)
			return
		}
	}

	u, err := url.Parse(s.env.URL)

	if err != nil {
		return
	}

	m, err := url.ParseQuery(u.RawQuery)

	if err != nil {
		return
	}

	if v, ok := m[prevSessionParam]; ok {
		psid = v[0]
	}

	return
}

func (s *Session) disconnectFromNode() {
	s.mu.Lock()
	if s.Connected {
		defer s.executor.Disconnect(s) // nolint:errcheck
	}
	s.Connected = false
	s.mu.Unlock()
}

func (s *Session) disconnectNow(reason string, code int) {
	s.disconnectFromNode()
	s.writeFrame(&ws.SentFrame{ // nolint:errcheck
		FrameType:   ws.CloseFrame,
		CloseReason: reason,
		CloseCode:   code,
	})

	s.mu.Lock()
	if s.sendCh != nil {
		close(s.sendCh)
		s.sendCh = nil
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

	if s.sendCh == nil {
		s.mu.Unlock()
		return
	}

	select {
	case s.sendCh <- message:
	default:
		if s.sendCh != nil {
			close(s.sendCh)
			defer s.Disconnect("Write failed", ws.CloseAbnormalClosure)
		}

		s.sendCh = nil
	}

	s.mu.Unlock()
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
		return errors.New("Closed")
	default:
		s.Log.Errorf("Unknown frame type: %v", message)
		return errors.New("Unknown frame type")
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
		s.Log.Errorf("Failed to encode ping message: %v", err)
	} else if b != nil {
		err = s.writeFrameWithDeadline(b, deadline)
	}

	if err != nil {
		s.Disconnect("Ping failed", ws.CloseAbnormalClosure)
		return
	}

	s.addPing()
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
