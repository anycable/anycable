package identity

import (
	"errors"
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/mocks"
	"github.com/stretchr/testify/assert"
)

func TestIdentifiableController(t *testing.T) {
	controller := mocks.Controller{}
	identifier := mocks.Identifier{}
	env := common.NewSessionEnv("ws://demo.anycable.io/cable", &map[string]string{"cookie": "val=1;"})
	commandResult := &common.CommandResult{Transmissions: []string{"message_sent"}, Streams: []string{"chat_42"}}

	subject := NewIdentifiableController(&controller, &identifier)

	t.Run("Start", func(t *testing.T) {
		controller.On("Start").Return(nil)

		assert.Nil(t, subject.Start())

		controller.AssertCalled(t, "Start")
	})

	t.Run("Shutdown", func(t *testing.T) {
		controller.On("Shutdown").Return(nil)

		assert.Nil(t, subject.Shutdown())

		controller.AssertCalled(t, "Shutdown")
	})

	t.Run("Authenticate (success)", func(t *testing.T) {
		expected := &common.ConnectResult{
			Identifier:         "test_ids",
			Transmissions:      []string{`{"type":"welcome","sid":"2021"}`},
			DisconnectInterest: -1,
			Status:             common.SUCCESS,
		}

		controller.On("Authenticate", "2021", env).Return(nil, errors.New("shouldn't be called"))
		identifier.On("Identify", "2021", env).Return(expected, nil)

		res, err := subject.Authenticate("2021", env)

		assert.NoError(t, err)
		assert.Equal(t, expected, res)
		controller.AssertNotCalled(t, "Authenticate", "2021", env)
	})

	t.Run("Authenticate (failure)", func(t *testing.T) {
		expected := &common.ConnectResult{Status: common.FAILURE}

		controller.On("Authenticate", "2020", env).Return(nil, errors.New("shouldn't be called"))
		identifier.On("Identify", "2020", env).Return(expected, nil)

		res, err := subject.Authenticate("2020", env)

		assert.NoError(t, err)
		assert.Equal(t, expected, res)
		controller.AssertNotCalled(t, "Authenticate", "2020", env)
	})

	t.Run("Authenticate (error)", func(t *testing.T) {
		expectedErr := errors.New("identifier failed")

		controller.On("Authenticate", "1998", env).Return(nil, errors.New("shouldn't be called"))
		identifier.On("Identify", "1998", env).Return(nil, expectedErr)

		res, err := subject.Authenticate("1998", env)

		assert.Nil(t, res)
		assert.Equal(t, expectedErr, err)
		controller.AssertNotCalled(t, "Authenticate", "1998", env)

	})

	t.Run("Authenticate (noop -> passthrough)", func(t *testing.T) {
		expected := &common.ConnectResult{Identifier: "test_ids", Transmissions: []string{`{"type":"welcome","sid":"2022"}`}, Status: common.SUCCESS}

		controller.On("Authenticate", "2022", env).Return(expected, nil)
		identifier.On("Identify", "2022", env).Return(nil, nil)

		res, err := subject.Authenticate("2022", env)

		assert.NoError(t, err)
		assert.Equal(t, expected, res)
		controller.AssertCalled(t, "Authenticate", "2022", env)
	})

	t.Run("Subscribe", func(t *testing.T) {
		controller.On("Subscribe", "42", env, "name=jack", "chat").Return(commandResult, nil)

		res, err := subject.Subscribe("42", env, "name=jack", "chat")

		assert.NoError(t, err)
		assert.Equal(t, commandResult, res)

		controller.AssertCalled(t, "Subscribe", "42", env, "name=jack", "chat")
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		controller.On("Unsubscribe", "42", env, "name=jack", "chat").Return(commandResult, nil)

		res, err := subject.Unsubscribe("42", env, "name=jack", "chat")

		assert.NoError(t, err)
		assert.Equal(t, commandResult, res)

		controller.AssertCalled(t, "Unsubscribe", "42", env, "name=jack", "chat")
	})

	t.Run("Perform", func(t *testing.T) {
		controller.On("Perform", "42", env, "name=jack", "chat", "ping").Return(commandResult, nil)

		res, err := subject.Perform("42", env, "name=jack", "chat", "ping")

		assert.NoError(t, err)
		assert.Equal(t, commandResult, res)

		controller.AssertCalled(t, "Perform", "42", env, "name=jack", "chat", "ping")
	})

	t.Run("Disconnect", func(t *testing.T) {
		expectedErr := errors.New("foo")
		controller.On("Disconnect", "42", env, "name=jack", []string{"chat"}).Return(expectedErr)

		err := subject.Disconnect("42", env, "name=jack", []string{"chat"})

		assert.Equal(t, expectedErr, err)

		controller.AssertCalled(t, "Disconnect", "42", env, "name=jack", []string{"chat"})
	})
}

func TestIdentifierPipeline(t *testing.T) {
	mocked := mocks.Identifier{}
	pub := NewPublicIdentifier()

	pipe := NewIdentifierPipeline(&mocked, pub)

	env := common.NewSessionEnv("ws://demo.anycable.io/cable", &map[string]string{"cookie": "val=1;"})

	t.Run("first identifier success", func(t *testing.T) {
		mocked.On("Identify", "mock-ok", env).Return(
			&common.ConnectResult{Identifier: "mock_welcome"},
			nil,
		)

		res, err := pipe.Identify("mock-ok", env)

		assert.NoError(t, err)
		assert.Equal(t, "mock_welcome", res.Identifier)
	})

	t.Run("second identifier success", func(t *testing.T) {
		mocked.On("Identify", "mock-nok", env).Return(nil, nil)

		res, err := pipe.Identify("mock-nok", env)

		assert.NoError(t, err)
		assert.Equal(t, `{"sid":"mock-nok"}`, res.Identifier)
	})

	t.Run("first identifier fail", func(t *testing.T) {
		mocked.On("Identify", "mock-err", env).Return(nil, errors.New("failed"))

		_, err := pipe.Identify("mock-err", env)

		assert.Error(t, err)
	})
}
