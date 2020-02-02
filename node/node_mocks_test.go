package node

import (
	"errors"

	"github.com/anycable/anycable-go/common"
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
	config := NewDisconnectQueueConfig()
	config.Rate = 1
	node.disconnector = NewDisconnectQueue(&node, &config)
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
		env:           common.NewSessionEnv("/cable-test", &map[string]string{}),
	}
}

// NewMockSession returns a new session with a specified uid, path and headers, and identifiers equal to uid
func NewMockSessionWithEnv(uid string, url string, headers *map[string]string) *Session {
	session := NewMockSession(uid)
	session.env = common.NewSessionEnv(url, headers)
	return session
}

// NewMockResult builds a new result with sid as transmission
func NewMockResult(sid string) *common.CommandResult {
	return &common.CommandResult{Transmissions: []string{sid}, Disconnect: false, StopAllStreams: false}
}

// Shutdown changes Started to false
func (c *MockController) Shutdown() error {
	c.Started = false
	return nil
}

// Authenticate emulates authentication process:
// - if path is equal to "failure" then authentication failed
// - otherwise returns value of headers['id'] as identifier
func (c *MockController) Authenticate(sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	if env.URL == "/failure" {
		return &common.ConnectResult{Transmissions: []string{"unauthorized"}}, errors.New("Auth Failed")
	}

	res := common.ConnectResult{Identifier: (*env.Headers)["id"], Transmissions: []string{"welcome"}}

	if (*env.Headers)["x-session-test"] != "" {
		res.CState = map[string]string{"_s_": (*env.Headers)["x-session-test"]}
	}

	return &res, nil
}

// Subscribe emulates subscription process:
// - if channel is equal to "failure" then returns subscription error
// - if channel is equal to "disconnect" then returns result with disconnect set to true
// - if channel is equal to "stream" then add "stream" to result.Streams
// - otherwise returns success result with one transmission equal to sid
func (c *MockController) Subscribe(sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
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
func (c *MockController) Unsubscribe(sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
	if channel == "failure" {
		return nil, errors.New("Unsubscription Failure")
	}

	res := NewMockResult(sid)
	return res, nil
}

// Perform return result with Transmissions containing data (i.e. emulates "echo" action)
func (c *MockController) Perform(sid string, env *common.SessionEnv, id string, channel string, data string) (*common.CommandResult, error) {
	if channel == "failure" {
		return nil, errors.New("Perform Failure")
	}

	res := NewMockResult(sid)
	res.Transmissions = []string{data}

	if data == "session" {
		res.CState = map[string]string{"_s_": "performed"}
	}

	return res, nil
}

func (c *MockController) Disconnect(sid string, env *common.SessionEnv, id string, subscriptions []string) error {
	return nil
}
