package node

import (
	"log/slog"

	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mocks"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/anycable/anycable-go/ws"
)

// NewMockNode build new node with mock controller
func NewMockNode() *Node {
	controller := mocks.NewMockController()
	config := NewConfig()
	config.HubGopoolSize = 2
	node := NewNode(&config, WithInstrumenter(metrics.NewMetrics(nil, 10, slog.Default())), WithController(&controller))
	node.SetBroker(broker.NewLegacyBroker(pubsub.NewLegacySubscriber(node)))
	dconfig := NewDisconnectQueueConfig()
	dconfig.Rate = 1
	node.SetDisconnector(NewDisconnectQueue(node, &dconfig, slog.Default()))
	return node
}

// NewMockSession returns a new session with a specified uid and identifiers equal to uid
func NewMockSession(uid string, node *Node, opts ...SessionOption) *Session {
	session := Session{
		executor:      node,
		closed:        true,
		uid:           uid,
		Log:           slog.With("sid", uid),
		subscriptions: NewSubscriptionState(),
		env:           common.NewSessionEnv("/cable-test", &map[string]string{}),
		sendCh:        make(chan *ws.SentFrame, 256),
		encoder:       encoders.JSON{},
		metrics:       metrics.NoopMetrics{},
		timers:        &SessionTimers{},
	}

	session.SetIdentifiers(uid)
	session.conn = mocks.NewMockConnection()

	for _, opt := range opts {
		opt(&session)
	}

	go session.SendMessages()

	return &session
}

// NewMockSession returns a new session with a specified uid, path and headers, and identifiers equal to uid
func NewMockSessionWithEnv(uid string, node *Node, url string, headers *map[string]string, opts ...SessionOption) *Session {
	session := NewMockSession(uid, node, opts...)
	session.env = common.NewSessionEnv(url, headers)
	return session
}
