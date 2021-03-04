package node

import (
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mocks"

	"github.com/apex/log"
)

// NewMockNode build new node with mock controller
func NewMockNode() Node {
	controller := mocks.NewMockController()
	config := NewConfig()
	node := NewNode(&controller, metrics.NewMetrics(nil, 10), &config)

	dconfig := NewDisconnectQueueConfig()
	dconfig.Rate = 1
	node.SetDisconnector(NewDisconnectQueue(node, &dconfig))
	return *node
}

// NewMockSession returns a new session with a specified uid and identifiers equal to uid
func NewMockSession(uid string) *Session {
	return &Session{
		send:          make(chan sentFrame, 256),
		closed:        true,
		UID:           uid,
		Identifiers:   uid,
		Log:           log.WithField("sid", uid),
		subscriptions: make(map[string]bool),
		env:           common.NewSessionEnv("/cable-test", &map[string]string{}),
	}
}

// NewMockSession returns a new session with a specified uid, path and headers, and identifiers equal to uid
func NewMockSessionWithEnv(uid string, url string, headers *map[string]string) *Session {
	session := NewMockSession(uid)
	session.env = common.NewSessionEnv(url, headers)
	return session
}
