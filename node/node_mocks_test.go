package node

import (
	"errors"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mocks"
	"github.com/anycable/anycable-go/ws"

	"github.com/apex/log"
)

// NewMockNode build new node with mock controller
func NewMockNode() *Node {
	controller := mocks.NewMockController()
	config := NewConfig()
	config.HubGopoolSize = 2
	node := NewNode(&controller, metrics.NewMetrics(nil, 10), &config)
	dconfig := NewDisconnectQueueConfig()
	dconfig.Rate = 1
	node.SetDisconnector(NewDisconnectQueue(node, &dconfig))
	return node
}

type MockConnection struct {
	session *Session
	send    chan []byte
	closed  bool
}

func (conn MockConnection) Write(msg []byte, deadline time.Time) error {
	conn.send <- msg
	return nil
}

func (conn MockConnection) WriteBinary(msg []byte, deadline time.Time) error {
	conn.send <- msg
	return nil
}

func (conn MockConnection) Read() ([]byte, error) {
	timer := time.After(100 * time.Millisecond)

	done := make(chan struct{}, 1)

	// Lazily retrieve pending session messages
	go func() {
		for {
			select {
			case <-done:
				return
			case frame := <-conn.session.sendCh:
				conn.session.writeFrame(frame) // nolint:errcheck
			}
		}
	}()

	defer func() {
		done <- struct{}{}
	}()

	select {
	case <-timer:
		return nil, errors.New("Session hasn't received any messages")
	case msg := <-conn.send:
		return msg, nil
	}
}

func (conn MockConnection) ReadIndifinitely() []byte {
	done := make(chan struct{}, 1)

	// Lazily retrieve pending session messages
	go func() {
		for {
			select {
			case <-done:
				return
			case frame := <-conn.session.sendCh:
				conn.session.writeFrame(frame) // nolint:errcheck
			}
		}
	}()

	defer func() {
		done <- struct{}{}
	}()

	msg := <-conn.send
	return msg
}

func (conn MockConnection) Close(_code int, _reason string) {
	conn.send <- []byte("")
}

func NewMockConnection(session *Session) MockConnection {
	return MockConnection{closed: false, send: make(chan []byte, 2), session: session}
}

// NewMockSession returns a new session with a specified uid and identifiers equal to uid
func NewMockSession(uid string, node *Node) *Session {
	session := Session{
		node:          node,
		closed:        true,
		UID:           uid,
		Identifiers:   uid,
		Log:           log.WithField("sid", uid),
		subscriptions: make(map[string]bool),
		env:           common.NewSessionEnv("/cable-test", &map[string]string{}),
		sendCh:        make(chan *ws.SentFrame, 256),
		encoder:       encoders.JSON{},
	}

	session.conn = NewMockConnection(&session)

	return &session
}

// NewMockSession returns a new session with a specified uid, path and headers, and identifiers equal to uid
func NewMockSessionWithEnv(uid string, node *Node, url string, headers *map[string]string) *Session {
	session := NewMockSession(uid, node)
	session.env = common.NewSessionEnv(url, headers)
	return session
}
