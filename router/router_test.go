package router

import (
	"errors"
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouterControllerWithoutRoutes(t *testing.T) {
	controller := mocks.Controller{}
	env := common.NewSessionEnv("ws://demo.anycable.io/cable", &map[string]string{"cookie": "val=1;"})

	subject := NewRouterController(&controller)

	t.Run("Authenticate", func(t *testing.T) {
		expected := &common.ConnectResult{Identifier: "test_ids", Transmissions: []string{"{\"type\":\"welcome\"}"}, Status: common.SUCCESS}

		controller.On("Authenticate", "2022", env).Return(expected, nil)

		res, err := subject.Authenticate("2022", env)

		assert.NoError(t, err)
		assert.Equal(t, expected, res)
		controller.AssertCalled(t, "Authenticate", "2022", env)
	})

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

	t.Run("Disconnect", func(t *testing.T) {
		expectedErr := errors.New("foo")
		controller.On("Disconnect", "42", env, "name=jack", []string{"chat"}).Return(expectedErr)

		err := subject.Disconnect("42", env, "name=jack", []string{"chat"})

		assert.Equal(t, expectedErr, err)

		controller.AssertCalled(t, "Disconnect", "42", env, "name=jack", []string{"chat"})
	})
}

func TestRouterControllerWithRoutes(t *testing.T) {
	controller := mocks.Controller{}
	env := common.NewSessionEnv("ws://demo.anycable.io/cable", &map[string]string{"cookie": "val=1;"})
	commandResult := &common.CommandResult{Transmissions: []string{"message_sent"}, Streams: []string{"missing"}}

	subject := NewRouterController(&controller)

	chatController := mocks.Controller{}
	chatResult := &common.CommandResult{Transmissions: []string{"message_sent"}, Streams: []string{"chat_42"}}
	require.NoError(t, subject.Route("ChatChannel", &chatController))

	echoController := mocks.Controller{}
	require.NoError(t, subject.Route("EchoChannel", &echoController))
	echoResult := &common.CommandResult{Transmissions: []string{"message_sent"}, Streams: []string{"hi"}}

	t.Run("Subscribe (channel with params)", func(t *testing.T) {
		channel := "{\"channel\":\"ChatChannel\",\"id\":\"42\"}"

		controller.On("Subscribe", "42", env, "name=jack", channel).Return(nil, errors.New("Shouldn't be called"))
		chatController.On("Subscribe", "42", env, "name=jack", channel).Return(chatResult, nil)

		res, err := subject.Subscribe("42", env, "name=jack", channel)

		require.NoError(t, err)
		assert.Equal(t, chatResult, res)
	})

	t.Run("Subscribe (fallback)", func(t *testing.T) {
		controller.On("Subscribe", "42", env, "name=jack", "fallback").Return(commandResult, nil)

		res, err := subject.Subscribe("42", env, "name=jack", "fallback")

		require.NoError(t, err)
		assert.Equal(t, commandResult, res)

		controller.AssertCalled(t, "Subscribe", "42", env, "name=jack", "fallback")
	})

	t.Run("Subscribe (pass)", func(t *testing.T) {
		channel := "{\"channel\":\"ChatChannel\",\"id\":\"2021\"}"

		controller.On("Subscribe", "42", env, "name=jack", channel).Return(commandResult, nil)
		chatController.On("Subscribe", "42", env, "name=jack", channel).Return(nil, nil)

		res, err := subject.Subscribe("42", env, "name=jack", channel)

		require.NoError(t, err)
		assert.Equal(t, commandResult, res)

		controller.AssertCalled(t, "Subscribe", "42", env, "name=jack", channel)
	})

	t.Run("Unsubscribe (fallback)", func(t *testing.T) {
		controller.On("Unsubscribe", "42", env, "name=jack", "fallback").Return(commandResult, nil)

		res, err := subject.Unsubscribe("42", env, "name=jack", "fallback")

		require.NoError(t, err)
		assert.Equal(t, commandResult, res)

		controller.AssertCalled(t, "Unsubscribe", "42", env, "name=jack", "fallback")
	})

	t.Run("Unsubscribe (pass)", func(t *testing.T) {
		channel := "{\"channel\":\"ChatChannel\",\"id\":\"2021\"}"

		controller.On("Unsubscribe", "42", env, "name=jack", channel).Return(commandResult, nil)
		chatController.On("Unsubscribe", "42", env, "name=jack", channel).Return(nil, nil)

		res, err := subject.Unsubscribe("42", env, "name=jack", channel)

		require.NoError(t, err)
		assert.Equal(t, commandResult, res)

		controller.AssertCalled(t, "Unsubscribe", "42", env, "name=jack", channel)
	})

	t.Run("Unsubscribe (channel with params)", func(t *testing.T) {
		channel := "{\"channel\":\"ChatChannel\",\"id\":\"42\"}"

		controller.On("Unsubscribe", "42", env, "name=jack", channel).Return(nil, errors.New("Shouldn't be called"))
		chatController.On("Unsubscribe", "42", env, "name=jack", channel).Return(chatResult, nil)

		res, err := subject.Unsubscribe("42", env, "name=jack", channel)

		require.NoError(t, err)
		assert.Equal(t, chatResult, res)
	})

	t.Run("Perform (channel w/o params)", func(t *testing.T) {
		channel := "{\"channel\":\"EchoChannel\"}"

		controller.On("Perform", "42", env, "name=jack", channel, "ping").Return(nil, errors.New("Shouldn't be called"))
		echoController.On("Perform", "42", env, "name=jack", channel, "ping").Return(echoResult, nil)

		res, err := subject.Perform("42", env, "name=jack", channel, "ping")

		require.NoError(t, err)
		assert.Equal(t, echoResult, res)
	})

	t.Run("Perform (fallback)", func(t *testing.T) {
		controller.On("Perform", "42", env, "name=jack", "fallback", "ping").Return(commandResult, nil)

		res, err := subject.Perform("42", env, "name=jack", "fallback", "ping")

		require.NoError(t, err)
		assert.Equal(t, commandResult, res)

		controller.AssertCalled(t, "Perform", "42", env, "name=jack", "fallback", "ping")
	})

	t.Run("Perform (pass)", func(t *testing.T) {
		channel := "{\"channel\":\"EchoChannel\"}"

		controller.On("Perform", "42", env, "name=jack", channel, "pass").Return(commandResult, nil)
		echoController.On("Perform", "42", env, "name=jack", channel, "pass").Return(nil, nil)

		res, err := subject.Perform("42", env, "name=jack", channel, "pass")

		require.NoError(t, err)
		assert.Equal(t, commandResult, res)

		controller.AssertCalled(t, "Perform", "42", env, "name=jack", channel, "pass")
	})
}
