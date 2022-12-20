package node

import (
	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mocks"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/anycable/anycable-go/ws"

	"github.com/apex/log"
)

// NewMockNode build new node with mock controller
func NewMockNode() *Node {
	controller := mocks.NewMockController()
	config := NewConfig()
	config.HubGopoolSize = 2
	node := NewNode(&controller, metrics.NewMetrics(nil, 10), &config)
	node.SetBroker(broker.NewLegacyBroker(pubsub.NewLegacySubscriber(node)))
	dconfig := NewDisconnectQueueConfig()
	dconfig.Rate = 1
	node.SetDisconnector(NewDisconnectQueue(node, &dconfig))
	return node
}

// NewMockSession returns a new session with a specified uid and identifiers equal to uid
func NewMockSession(uid string, node *Node) *Session {
	session := Session{
		executor:      node,
		closed:        true,
		uid:           uid,
		Log:           log.WithField("sid", uid),
		subscriptions: NewSubscriptionState(),
		env:           common.NewSessionEnv("/cable-test", &map[string]string{}),
		sendCh:        make(chan *ws.SentFrame, 256),
		encoder:       encoders.JSON{},
		metrics:       metrics.NoopMetrics{},
	}

	session.SetIdentifiers(uid)
	session.conn = mocks.NewMockConnection()
	go session.SendMessages()

	return &session
}

// NewMockSession returns a new session with a specified uid, path and headers, and identifiers equal to uid
func NewMockSessionWithEnv(uid string, node *Node, url string, headers *map[string]string) *Session {
	session := NewMockSession(uid, node)
	session.env = common.NewSessionEnv(url, headers)
	return session
}
