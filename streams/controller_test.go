package streams

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	key = "s3Krit"
	// Turbo.signed_stream_verifier_key = 's3Krit'
	// Turbo::StreamsChannel.signed_stream_name([:chat, "2021"])
	stream = "ImNoYXQ6MjAyMSI=--f9ee45dbccb1da04d8ceb99cc820207804370ba0d06b46fc3b8b373af1315628"
)

func TestNewController(t *testing.T) {
	t.Run("No stream name", func(t *testing.T) {
		resolver := func(string) (*SubscribeRequest, error) {
			return &SubscribeRequest{}, nil
		}

		subject := NewController(key, resolver, slog.Default())

		require.NotNil(t, subject)

		res, err := subject.Subscribe("42", nil, "name=jack", "")

		require.Error(t, err)
		require.NotNil(t, res)

		assert.Equal(t, common.FAILURE, res.Status)
		assert.Equal(t, []string{common.RejectionMessage("")}, res.Transmissions)
	})
}

func TestStreamsController(t *testing.T) {
	t.Run("Subscribe - public", func(t *testing.T) {
		conf := NewConfig()
		conf.Public = true
		subject := NewStreamsController(&conf, slog.Default())

		require.NotNil(t, subject)

		res, err := subject.Subscribe("42", nil, "name=jack", `{"channel":"$pubsub","stream_name":"chat:2024"}`)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{common.ConfirmationMessage(`{"channel":"$pubsub","stream_name":"chat:2024"}`)}, res.Transmissions)
		assert.Equal(t, []string{"chat:2024"}, res.Streams)
		assert.Equal(t, -1, res.DisconnectInterest)
		assert.Equal(t, "chat:2024", res.IState[common.WHISPER_STREAM_STATE])
	})

	t.Run("Subscribe - no public allowed", func(t *testing.T) {
		conf := NewConfig()
		subject := NewStreamsController(&conf, slog.Default())

		require.NotNil(t, subject)

		res, err := subject.Subscribe("42", nil, "name=jack", `{"channel":"$pubsub","stream_name":"chat:2024"}`)

		require.Error(t, err)
		require.NotNil(t, res)
		assert.Equal(t, []string{common.RejectionMessage(`{"channel":"$pubsub","stream_name":"chat:2024"}`)}, res.Transmissions)
	})

	t.Run("Subscribe - signed", func(t *testing.T) {
		conf := NewConfig()
		conf.Secret = key
		conf.Presence = false
		subject := NewStreamsController(&conf, slog.Default())

		require.NotNil(t, subject)

		identifier := `{"channel":"$pubsub","signed_stream_name":"` + stream + `"}`

		res, err := subject.Subscribe("42", nil, "name=jack", identifier)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{common.ConfirmationMessage(identifier)}, res.Transmissions)
		assert.Equal(t, []string{"chat:2021"}, res.Streams)
		assert.Equal(t, -1, res.DisconnectInterest)
		assert.Nil(t, res.IState)
	})

	t.Run("Subscribe - signed - no key", func(t *testing.T) {
		conf := NewConfig()
		conf.Secret = ""
		conf.Presence = false
		subject := NewStreamsController(&conf, slog.Default())

		require.NotNil(t, subject)

		identifier := `{"channel":"$pubsub","signed_stream_name":"` + stream + `"}`

		res, err := subject.Subscribe("42", nil, "name=jack", identifier)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.FAILURE, res.Status)
		assert.Equal(t, []string{common.RejectionMessage(identifier)}, res.Transmissions)
	})

	t.Run("Subscribe - signed - whisper", func(t *testing.T) {
		conf := NewConfig()
		conf.Secret = key
		conf.Whisper = true
		conf.Presence = false
		subject := NewStreamsController(&conf, slog.Default())

		require.NotNil(t, subject)

		identifier := `{"channel":"$pubsub","signed_stream_name":"` + stream + `"}`

		res, err := subject.Subscribe("42", nil, "name=jack", identifier)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{common.ConfirmationMessage(identifier)}, res.Transmissions)
		assert.Equal(t, []string{"chat:2021"}, res.Streams)
		assert.Equal(t, -1, res.DisconnectInterest)
		assert.Equal(t, "chat:2021", res.IState[common.WHISPER_STREAM_STATE])
	})
}

func TestTurboController(t *testing.T) {
	env := common.NewSessionEnv("ws://demo.anycable.io/cable", &map[string]string{"cookie": "val=1;"})
	subject := NewTurboController(key, slog.Default())

	t.Run("Subscribe (success)", func(t *testing.T) {
		channel := fmt.Sprintf("{\"channel\":\"Turbo::StreamsChannel\",\"signed_stream_name\":\"%s\"}", stream)

		res, err := subject.Subscribe("42", env, "name=jack", channel)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{common.ConfirmationMessage(channel)}, res.Transmissions)
		assert.Equal(t, []string{"chat:2021"}, res.Streams)
		assert.Equal(t, -1, res.DisconnectInterest)
	})

	t.Run("Subscribe (failure)", func(t *testing.T) {
		channel := fmt.Sprintf("{\"channel\":\"Turbo::StreamsChannel\",\"signed_stream_name\":\"%s\"}", "fake_id")

		res, err := subject.Subscribe("42", env, "name=jack", channel)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.FAILURE, res.Status)
		assert.Equal(t, []string{common.RejectionMessage(channel)}, res.Transmissions)
	})

	t.Run("Subscribe (failure + not a string)", func(t *testing.T) {
		signed := "WyJjaGF0LzIwMjMiLDE2ODUwMjQwMTdd--5b6661024d4c463c4936cd1542bc9a7672dd8039ac407d0b6c901697190e8aeb"
		channel := fmt.Sprintf("{\"channel\":\"Turbo::StreamsChannel\",\"signed_stream_name\":\"%s\"}", signed)

		res, err := subject.Subscribe("42", env, "name=jack", channel)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.FAILURE, res.Status)
		assert.Equal(t, []string{common.RejectionMessage(channel)}, res.Transmissions)
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		channel := fmt.Sprintf("{\"channel\":\"Turbo::StreamsChannel\",\"signed_stream_name\":\"%s\"}", stream)

		res, err := subject.Unsubscribe("42", env, "name=jack", channel)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{}, res.Transmissions)
		assert.Equal(t, []string{}, res.Streams)
		assert.Equal(t, true, res.StopAllStreams)
	})
}

func TestCableReadyController(t *testing.T) {
	env := common.NewSessionEnv("ws://demo.anycable.io/cable", &map[string]string{"cookie": "val=1;"})
	subject := NewCableReadyController(key, slog.Default())

	t.Run("Subscribe (success)", func(t *testing.T) {
		channel := fmt.Sprintf("{\"channel\":\"CableReady::Stream\",\"identifier\":\"%s\"}", stream)

		res, err := subject.Subscribe("42", env, "name=jack", channel)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{common.ConfirmationMessage(channel)}, res.Transmissions)
		assert.Equal(t, []string{"chat:2021"}, res.Streams)
		assert.Equal(t, -1, res.DisconnectInterest)
	})

	t.Run("Subscribe (failure)", func(t *testing.T) {
		channel := fmt.Sprintf("{\"channel\":\"CableReady::Stream\",\"identifier\":\"%s\"}", "fake_id")

		res, err := subject.Subscribe("42", env, "name=jack", channel)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.FAILURE, res.Status)
		assert.Equal(t, []string{common.RejectionMessage(channel)}, res.Transmissions)
	})

	t.Run("Subscribe (failure + not a string)", func(t *testing.T) {
		signed := "WyJjaGF0LzIwMjMiLDE2ODUwMjQwMTdd--5b6661024d4c463c4936cd1542bc9a7672dd8039ac407d0b6c901697190e8aeb"
		channel := fmt.Sprintf("{\"channel\":\"CableReady::Stream\",\"identifier\":\"%s\"}", signed)

		res, err := subject.Subscribe("42", env, "name=jack", channel)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.FAILURE, res.Status)
		assert.Equal(t, []string{common.RejectionMessage(channel)}, res.Transmissions)
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		channel := fmt.Sprintf("{\"channel\":\"CableReady::Stream\",\"identifier\":\"%s\"}", stream)

		res, err := subject.Unsubscribe("42", env, "name=jack", channel)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{}, res.Transmissions)
		assert.Equal(t, []string{}, res.Streams)
		assert.Equal(t, true, res.StopAllStreams)
	})
}
