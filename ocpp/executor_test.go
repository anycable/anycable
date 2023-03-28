package ocpp

import (
	"errors"
	"strconv"
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mocks"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/node_mocks"
	"github.com/apex/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHandleCommand(t *testing.T) {
	app := &node_mocks.AppNode{}
	n := NewMockNode()
	c := NewConfig()
	executor := NewExecutor(app, &c)

	app.On("Disconnect", mock.Anything).Return(nil)

	t.Run("boot notification success + ack", func(t *testing.T) {
		conn := mocks.NewMockConnection()
		session := buildSession(conn, n, executor)
		defer session.Disconnect("test", 0)
		session.Connected = false

		app.On("Authenticate", mock.MatchedBy(node_mocks.SessionMatcher(session)), mock.Anything).Return(&common.ConnectResult{}, nil)

		subscribe := common.Message{Command: "subscribe", Identifier: IDToIdentifier("EV42", "OCPPChannel")}
		perform := common.Message{Command: "message", Identifier: IDToIdentifier("EV42", "OCPPChannel"), Data: `{"action":"boot_notification","command":"BootNotification","id":"22","payload":{"meterSerialNumber":"EV42"}}`}

		app.On("Subscribe", mock.MatchedBy(node_mocks.SessionMatcher(session)), &subscribe).Return(&common.CommandResult{}, nil)
		app.On("Perform", mock.MatchedBy(node_mocks.SessionMatcher(session)), &perform).Return(&common.CommandResult{}, nil)

		err := executor.HandleCommand(
			session, &common.Message{
				Command:    "BootNotification",
				Identifier: "22",
				Data: CallMessage{
					Payload: map[string]interface{}{
						"meterSerialNumber": "EV42",
					},
				},
			},
		)
		require.NoError(t, err)

		assert.Equal(t, "EV42", session.InternalState["sn"])

		res, err := conn.Read()
		require.NoError(t, err)

		assert.Equal(t, `[3,"22",{"interval":30,"status":"Accepted"}]`, string(res))
	})

	t.Run("boot notification error", func(t *testing.T) {
		conn := mocks.NewMockConnection()
		session := buildSession(conn, n, executor)
		defer session.Disconnect("test", 0)
		session.Connected = false

		expectedError := errors.New("Failed")
		app.On("Authenticate", mock.MatchedBy(node_mocks.SessionMatcher(session)), mock.Anything).Return(nil, expectedError)

		err := executor.HandleCommand(session, &common.Message{
			Command:    "BootNotification",
			Identifier: "20",
			Data: CallMessage{
				Payload: map[string]interface{}{
					"meterSerialNumber": "EV42",
				},
			},
		})

		assert.Equal(t, expectedError, err)

		_, err = conn.Read()
		require.Error(t, err)
	})

	t.Run("boot notification failure", func(t *testing.T) {
		conn := mocks.NewMockConnection()
		session := buildSession(conn, n, executor)
		defer session.Disconnect("test", 0)
		session.Connected = false

		app.On("Authenticate", mock.MatchedBy(node_mocks.SessionMatcher(session)), mock.Anything).Return(&common.ConnectResult{Status: common.FAILURE}, nil)

		err := executor.HandleCommand(session, &common.Message{
			Command:    "BootNotification",
			Identifier: "13",
			Data: CallMessage{
				Payload: map[string]interface{}{
					"meterSerialNumber": "EV444",
				},
			},
		})

		assert.NoError(t, err)

		res, err := conn.Read()
		require.NoError(t, err)

		assert.Equal(t, `[3,"13",{"status":"Rejected"}]`, string(res))
	})

	t.Run("boot notification when already connected sends error", func(t *testing.T) {
		conn := mocks.NewMockConnection()
		session := buildSession(conn, n, executor)
		defer session.Disconnect("test", 0)
		session.Connected = true

		app.On("Authenticate", mock.MatchedBy(node_mocks.SessionMatcher(session)), mock.Anything).Return(nil, nil)

		err := executor.HandleCommand(session, &common.Message{
			Command:    "BootNotification",
			Identifier: "23",
			Data: CallMessage{
				Payload: map[string]interface{}{
					"meterSerialNumber": "EV42",
				},
			},
		})

		assert.NoError(t, err)

		res, err := conn.Read()
		require.NoError(t, err)

		assert.Equal(t, `[4,"23","FormationViolation","Already booted",null]`, string(res))
	})

	t.Run("heartbeat", func(t *testing.T) {
		conn := mocks.NewMockConnection()
		session := buildSession(conn, n, executor)
		defer session.Disconnect("test", 0)

		err := executor.HandleCommand(session, &common.Message{
			Command:    "Heartbeat",
			Identifier: "hb43",
		})
		assert.NoError(t, err)

		res, err := conn.Read()
		require.NoError(t, err)

		assert.Contains(t, string(res), `[3,"hb43",{"currentTime":"`)
	})

	t.Run("ack", func(t *testing.T) {
		conn := mocks.NewMockConnection()
		session := buildSession(conn, n, executor)
		defer session.Disconnect("test", 0)

		perform := common.Message{
			Command:    "message",
			Identifier: IDToIdentifier("EV2022", "OCPPChannel"),
			Data:       `{"action":"ack","command":"Ack","id":"ab31","payload":{"status":"Accepted"}}`,
		}

		session.WriteInternalState("sn", "EV2022")

		app.On("Perform", mock.MatchedBy(node_mocks.SessionMatcher(session)), &perform).Return(&common.CommandResult{}, nil)

		err := executor.HandleCommand(
			session, &common.Message{
				Command:    AckCommand,
				Identifier: "ab31",
				Data: AckMessage{
					Payload: map[string]string{"status": "Accepted"},
				},
			})

		assert.NoError(t, err)

		// Ack should be double-acked
		_, err = conn.Read()
		require.Error(t, err)
	})

	t.Run("command", func(t *testing.T) {
		conn := mocks.NewMockConnection()
		session := buildSession(conn, n, executor)
		defer session.Disconnect("test", 0)

		session.WriteInternalState("sn", "EV2012")

		perform := common.Message{
			Command:    "message",
			Identifier: IDToIdentifier("EV2012", "OCPPChannel"),
			Data:       `{"action":"status_notification","command":"StatusNotification","id":"st42","payload":{"connectorId":1,"errorCode":"NoError","status":"Available"}}`,
		}

		app.On("Perform", mock.MatchedBy(node_mocks.SessionMatcher(session)), &perform).Return(&common.CommandResult{}, nil)

		err := executor.HandleCommand(
			session, &common.Message{
				Command:    "StatusNotification",
				Identifier: "st42",
				Data: CallMessage{
					Command: "StatusNotification",
					Payload: map[string]interface{}{"connectorId": 1, "errorCode": "NoError", "status": "Available"},
				},
			})

		assert.NoError(t, err)

		res, err := conn.Read()
		require.NoError(t, err)

		assert.Equal(t, `[3,"st42",{"status":"Accepted"}]`, string(res))
	})

	t.Run("command when no serial number stored", func(t *testing.T) {
		conn := mocks.NewMockConnection()
		session := buildSession(conn, n, executor)
		defer session.Disconnect("test", 0)

		err := executor.HandleCommand(
			session, &common.Message{
				Command:    "StatusNotification",
				Identifier: "sn0",
				Data: CallMessage{
					Command: "StatusNotification",
					Payload: map[string]interface{}{"connectorId": 1, "errorCode": "NoError", "status": "Available"},
				},
			})

		assert.Error(t, err)

		res, err := conn.Read()
		require.NoError(t, err)

		assert.Equal(t, `[4,"sn0","FormationViolation","No serial number",null]`, string(res))
	})
}

var (
	sessionCounter = 1
)

func buildSession(conn node.Connection, n *node.Node, executor node.Executor) *node.Session {
	sessionCounter++
	s := node.NewSession(n, conn, "ws://anycable.io/ocpp/43", nil, strconv.Itoa(sessionCounter), node.WithEncoder(Encoder{}), node.WithExecutor(executor))
	s.Connected = true
	s.Log = log.WithField("context", "test")
	return s
}

// NewMockNode build new node with mock controller
func NewMockNode() *node.Node {
	controller := mocks.NewMockController()
	config := node.NewConfig()
	config.HubGopoolSize = 2
	config.ReadGopoolSize = 2
	config.WriteGopoolSize = 2
	node := node.NewNode(&config, node.WithController(&controller), node.WithInstrumenter(metrics.NewMetrics(nil, 10, slog.Default())))
	return node
}
