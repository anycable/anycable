package node

import (
	"errors"
	"sync"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/ws"
	"github.com/apex/log"
)

const (
	writeWait = 10 * time.Second
)

// Executor handles incoming commands (messages)
type Executor interface {
	HandleCommand(*Session, *common.Message) error
}

// Session represents active client
type Session struct {
	node          *Node
	conn          Connection
	encoder       encoders.Encoder
	executor      Executor
	env           *common.SessionEnv
	subscriptions map[string]bool
	closed        bool
	// Main mutex (for read/write and important session updates)
	mu sync.Mutex
	// Mutex for protocol-related state (env, subscriptions)
	smu sync.Mutex

	sendCh chan *ws.SentFrame

	pingTimer    *time.Timer
	pingInterval time.Duration

	pingTimestampPrecision string

	UID         string
	Identifiers string
	Connected   bool
	// Could be used to store arbitrary data within a session
	InternalState map[string]interface{}
	Log           *log.Entry
}

// NewSession build a new Session struct from ws connetion and http request
func NewSession(node *Node, conn Connection, url string, headers *map[string]string, uid string) *Session {
	session := &Session{
		node:                   node,
		conn:                   conn,
		env:                    common.NewSessionEnv(url, headers),
		subscriptions:          make(map[string]bool),
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

	session.UID = uid

	ctx := node.log.WithFields(log.Fields{
		"sid": session.UID,
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

func (s *Session) GetEnv() *common.SessionEnv {
	return s.env
}

func (s *Session) SetEnv(env *common.SessionEnv) {
	s.env = env
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
			s.node.Metrics.Counter(metricsFailedSent).Inc()
			return
		}

		s.node.Metrics.Counter(metricsSentMsg).Inc()
	}
}

// ReadMessage reads messages from ws connection and send them to node
func (s *Session) ReadMessage(message []byte) error {
	s.node.Metrics.Counter(metricsDataReceived).Add(uint64(len(message)))

	command, err := s.decodeMessage(message)

	if err != nil {
		s.node.Metrics.Counter(metricsFailedCommandReceived).Inc()
		return err
	}

	if command == nil {
		return nil
	}

	s.node.Metrics.Counter(metricsReceivedMsg).Inc()

	if err := s.executor.HandleCommand(s, command); err != nil {
		s.node.Metrics.Counter(metricsFailedCommandReceived).Inc()
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

func (s *Session) disconnectFromNode() {
	s.mu.Lock()
	if s.Connected {
		defer s.node.Disconnect(s) // nolint:errcheck
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
	s.node.Metrics.Counter(metricsDataSent).Add(uint64(len(message.Payload)))

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
	if s.closed {
		return
	}

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
	if cm, ok := msg.(*CachedEncodedMessage); ok {
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
