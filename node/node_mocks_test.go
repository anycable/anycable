package node

import (
	"errors"

	"github.com/anycable/anycable-go/metrics"

	"github.com/apex/log"
)

// MockController implements controller interface for tests
type MockController struct {
	Started bool
}

// NewMockController builds new mock controller instance
func NewMockController() MockController {
	return MockController{Started: true}
}

// NewMockNode build new node with mock controller
func NewMockNode() Node {
	controller := NewMockController()
	node := Node{
		controller: &controller,
		hub:        NewHub(),
		Metrics:    metrics.NewMetrics(nil, 10),
		log:        log.WithField("context", "test"),
	}
	node.registerMetrics()
	node.disconnector = NewDisconnectQueue(&node, 1)
	return node
}

// NewMockSession returns a new session with a specified uid and identifiers equal to uid
func NewMockSession(uid string) *Session {
	return &Session{
		send:          make(chan []byte, 256),
		closed:        true,
		UID:           uid,
		Identifiers:   uid,
		Log:           log.WithField("sid", uid),
		subscriptions: make(map[string]bool),
	}
}

// NewMockResult builds a new result with sid as transmission
func NewMockResult(sid string) *CommandResult {
	return &CommandResult{Transmissions: []string{sid}, Disconnect: false, StopAllStreams: false}
}

// Shutdown changes Started to false
func (c *MockController) Shutdown() error {
	c.Started = false
	return nil
}

// Authenticate emulates authentication process:
// - if path is equal to "failure" then authentication failed
// - otherwise returns value of headers['id'] as identifier
func (c *MockController) Authenticate(sid string, path string, headers *map[string]string) (string, []string, error) {
	if path == "/failure" {
		return "", []string{"unauthorized"}, errors.New("Auth Failed")
	}

	return (*headers)["id"], []string{"welcome"}, nil
}

// Subscribe emulates subscription process:
// - if channel is equal to "failure" then returns subscription error
// - if channel is equal to "disconnect" then returns result with disconnect set to true
// - if channel is equal to "stream" then add "stream" to result.Streams
// - otherwise returns success result with one transmission equal to sid
func (c *MockController) Subscribe(sid string, id string, channel string) (*CommandResult, error) {
	if channel == "failure" {
		return nil, errors.New("Subscription Failure")
	}

	res := NewMockResult(sid)

	if channel == "disconnect" {
		res.Disconnect = true
		return res, nil
	}

	if channel == "stream" {
		res.Streams = []string{"stream"}
	}

	return res, nil
}

// Unsubscribe returns command result
func (c *MockController) Unsubscribe(sid string, id string, channel string) (*CommandResult, error) {
	if channel == "failure" {
		return nil, errors.New("Unsubscription Failure")
	}

	res := NewMockResult(sid)
	return res, nil
}

// Perform return result with Transmissions containing data (i.e. emulates "echo" action)
func (c *MockController) Perform(sid string, id string, channel string, data string) (*CommandResult, error) {
	if channel == "failure" {
		return nil, errors.New("Perform Failure")
	}

	res := NewMockResult(sid)
	res.Transmissions = []string{data}
	return res, nil
}

func (c *MockController) Disconnect(sid string, id string, subscriptions []string, path string, headers *map[string]string) error {
	return nil
}
