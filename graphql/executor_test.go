package graphql

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/node_mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleCommand(t *testing.T) {
	n := &node_mocks.AppNode{}
	c := NewConfig()
	c.JWTParam = "jtoken"
	executor := NewExecutor(n, &c)
	nodeConfig := node.NewConfig()
	hubNode := node.NewNode(&nodeConfig)

	n.On("Disconnect", mock.Anything).Return(nil)

	t.Run("connection_init success", func(t *testing.T) {
		session := buildSession(executor)
		n.On("Authenticate", session).Return(nil, nil)

		err := executor.HandleCommand(session, &common.Message{Command: ConnectionInitType})
		assert.NoError(t, err)
		n.AssertCalled(t, "Authenticate", session)
	})

	t.Run("connection_init with payload", func(t *testing.T) {
		session := buildSession(executor)
		n.On("Authenticate", session).Return(nil, nil)

		err := executor.HandleCommand(session, &common.Message{Command: ConnectionInitType, Data: "some_payload"})
		assert.NoError(t, err)
		n.AssertCalled(t, "Authenticate", session)
		assert.Equal(t, "some_payload", (*session.GetEnv().Headers)["x-apollo-connection"])
	})

	t.Run("connection_init with jwt token", func(t *testing.T) {
		session := buildSession(executor)
		n.On("Authenticate", session).Return(nil, nil)

		err := executor.HandleCommand(
			session,
			&common.Message{
				Command: ConnectionInitType,
				Data:    `{"jtoken":"secret-token"}`,
			},
		)
		assert.NoError(t, err)
		n.AssertCalled(t, "Authenticate", session)
		assert.Equal(t, "secret-token", (*session.GetEnv().Headers)["x-jtoken"])
	})

	t.Run("connection_init failure", func(t *testing.T) {
		session := buildSession(executor)
		expectedError := errors.New("Failed")
		n.On("Authenticate", session).Return(nil, expectedError)

		err := executor.HandleCommand(session, &common.Message{Command: ConnectionInitType})
		assert.Equal(t, expectedError, err)
		n.AssertCalled(t, "Authenticate", session)
	})

	t.Run("connection_init when already connected", func(t *testing.T) {
		session := buildSession(executor)
		hubNode.Authenticated(session, "")
		n.On("Authenticate", session).Return(nil, nil)

		err := executor.HandleCommand(session, &common.Message{Command: ConnectionInitType})
		assert.Error(t, err)
		n.AssertNotCalled(t, "Authenticate", session)
	})

	t.Run("ping", func(t *testing.T) {
		session := buildSession(executor)
		hubNode.Authenticated(session, "")

		err := executor.HandleCommand(session, &common.Message{Command: PingType})
		assert.NoError(t, err)
	})

	t.Run("pong", func(t *testing.T) {
		session := buildSession(executor)
		hubNode.Authenticated(session, "")

		err := executor.HandleCommand(session, &common.Message{Command: PongType})
		assert.NoError(t, err)
	})

	t.Run("start when not connected", func(t *testing.T) {
		session := buildSession(executor)
		n.On("Subscribe", session, mock.Anything).Return(nil, nil)

		err := executor.HandleCommand(session, &common.Message{Command: SubscribeType})
		assert.Error(t, err)
		n.AssertNotCalled(t, "Subscribe", session, mock.Anything)
	})

	t.Run("start with subscription", func(t *testing.T) {
		session := buildSession(executor)
		hubNode.Authenticated(session, "")
		gqlCommand := buildSubscribeCommand("{\"query\":\"User(id: 1){name}\"}")
		command := common.Message{Command: "subscribe", Identifier: IDToIdentifier(gqlCommand.Identifier, "GraphqlChannel")}
		perform := common.Message{Command: "message", Identifier: IDToIdentifier(gqlCommand.Identifier, "GraphqlChannel"), Data: "{\"query\":\"User(id: 1){name}\",\"action\":\"execute\"}"}
		unsubscribe := common.Message{Command: "unsubscribe", Identifier: IDToIdentifier(gqlCommand.Identifier, "GraphqlChannel")}
		result := common.CommandResult{Transmissions: []string{gqlTransmission(true)}}

		n.On("Subscribe", session, &command).Return(&common.CommandResult{}, nil)
		n.On("Perform", session, &perform).Return(&result, nil)
		n.On("Unsubscribe", session, &unsubscribe).Return(&common.CommandResult{}, nil)

		err := executor.HandleCommand(session, gqlCommand)
		assert.NoError(t, err)
		n.AssertCalled(t, "Subscribe", session, &command)
		n.AssertCalled(t, "Perform", session, &perform)
		n.AssertCalled(t, "Unsubscribe", session, &unsubscribe)
	})

	t.Run("start with subscription with custom channel and action", func(t *testing.T) {
		customConfig := NewConfig()
		customConfig.Channel = "MyGraphqlChannel"
		customConfig.Action = "run"

		customExec := NewExecutor(n, &customConfig)

		session := buildSession(customExec)
		hubNode.Authenticated(session, "")
		gqlCommand := buildSubscribeCommand("{\"query\":\"User(id: 1){name}\"}")
		command := common.Message{Command: "subscribe", Identifier: IDToIdentifier(gqlCommand.Identifier, "MyGraphqlChannel")}
		perform := common.Message{Command: "message", Identifier: IDToIdentifier(gqlCommand.Identifier, "MyGraphqlChannel"), Data: "{\"query\":\"User(id: 1){name}\",\"action\":\"run\"}"}
		unsubscribe := common.Message{Command: "unsubscribe", Identifier: IDToIdentifier(gqlCommand.Identifier, "MyGraphqlChannel")}
		result := common.CommandResult{Transmissions: []string{gqlTransmission(true)}}

		n.On("Subscribe", session, &command).Return(&common.CommandResult{}, nil)
		n.On("Perform", session, &perform).Return(&result, nil)
		n.On("Unsubscribe", session, &unsubscribe).Return(&common.CommandResult{}, nil)

		err := customExec.HandleCommand(session, gqlCommand)
		assert.NoError(t, err)
		n.AssertCalled(t, "Subscribe", session, &command)
		n.AssertCalled(t, "Perform", session, &perform)
		n.AssertCalled(t, "Unsubscribe", session, &unsubscribe)
	})

	t.Run("start with subscription failure", func(t *testing.T) {
		session := buildSession(executor)
		hubNode.Authenticated(session, "")
		gqlCommand := buildSubscribeCommand("{\"query\":\"User(id: 1){name}\"}")
		command := common.Message{Command: "subscribe", Identifier: IDToIdentifier(gqlCommand.Identifier, "GraphqlChannel")}
		expectedError := errors.New("Failure")
		n.On("Subscribe", session, &command).Return(nil, expectedError)
		n.On("Perform", session, mock.Anything).Return(nil, nil)

		err := executor.HandleCommand(session, gqlCommand)
		assert.Equal(t, expectedError, err)
		n.AssertCalled(t, "Subscribe", session, &command)
		n.AssertNotCalled(t, "Perform", session, mock.Anything)
	})

	t.Run("start with request", func(t *testing.T) {
		session := buildSession(executor)
		hubNode.Authenticated(session, "")
		gqlCommand := buildSubscribeCommand("{\"query\":\"User(id: 1){name}\"}")
		command := common.Message{Command: "subscribe", Identifier: IDToIdentifier(gqlCommand.Identifier, "GraphqlChannel")}
		perform := common.Message{Command: "message", Identifier: IDToIdentifier(gqlCommand.Identifier, "GraphqlChannel"), Data: "{\"query\":\"User(id: 1){name}\",\"action\":\"execute\"}"}
		unsubscribe := common.Message{Command: "unsubscribe", Identifier: IDToIdentifier(gqlCommand.Identifier, "GraphqlChannel")}
		result := common.CommandResult{Transmissions: []string{gqlTransmission(false)}}

		n.On("Subscribe", session, &command).Return(&common.CommandResult{}, nil)
		n.On("Perform", session, &perform).Return(&result, nil)
		n.On("Unsubscribe", session, &unsubscribe).Return(&common.CommandResult{}, nil)

		err := executor.HandleCommand(session, gqlCommand)
		assert.NoError(t, err)
		n.AssertCalled(t, "Subscribe", session, &command)
		n.AssertCalled(t, "Perform", session, &perform)
		n.AssertCalled(t, "Unsubscribe", session, &unsubscribe)
	})

	t.Run("complete", func(t *testing.T) {
		session := buildSession(executor)
		hubNode.Authenticated(session, "")
		gqlCommand := &common.Message{Command: CompleteType, Identifier: "stopMe"}
		unsubscribe := common.Message{Command: "unsubscribe", Identifier: IDToIdentifier(gqlCommand.Identifier, "GraphqlChannel")}

		n.On("Unsubscribe", session, &unsubscribe).Return(&common.CommandResult{}, nil)

		err := executor.HandleCommand(session, gqlCommand)
		assert.NoError(t, err)
		n.AssertCalled(t, "Unsubscribe", session, &unsubscribe)
	})
}

var (
	sessionCounter = 1
	commandCounter = 1
)

func buildSession(executor node.Executor) *node.Session {
	sessionCounter++
	s := node.BuildSession(nil, common.NewSessionEnv("ws://anycable.io/cable", nil))
	s.Log = slog.With("context", "test")
	s.SetID(strconv.Itoa(sessionCounter))
	node.WithEncoder(Encoder{})(&s)
	node.WithExecutor(executor)(&s)
	s.SetEnv(common.NewSessionEnv("ws://anycable.io/cable", nil))
	return &s
}

func buildSubscribeCommand(query string) *common.Message {
	commandCounter++
	return &common.Message{
		Command:    SubscribeType,
		Identifier: strconv.Itoa(commandCounter),
		Data:       query,
	}
}

func gqlTransmission(subscription bool) string {
	return fmt.Sprintf("{\"identifier\":\"noop\",\"type\":\"message\",\"data\":{\"more\":%v}}", subscription)
}
