package rails

import (
	"fmt"
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTurboController(t *testing.T) {
	key := "s3Krit"
	// Turbo.signed_stream_verifier_key = 's3Krit'
	// Turbo::StreamsChannel.signed_stream_name([:chat, "2021"])
	stream := "ImNoYXQ6MjAyMSI=--f9ee45dbccb1da04d8ceb99cc820207804370ba0d06b46fc3b8b373af1315628"

	env := common.NewSessionEnv("ws://demo.anycable.io/cable", &map[string]string{"cookie": "val=1;"})
	subject := NewTurboController(key)

	t.Run("Subscribe (success)", func(t *testing.T) {
		channel := fmt.Sprintf("{\"channel\":\"Turbo::StreamsChannel\",\"signed_stream_name\":\"%s\"}", stream)

		res, err := subject.Subscribe("42", env, "name=jack", channel)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{common.ConfirmationMessage(channel)}, res.Transmissions)
		assert.Equal(t, []string{"chat:2021"}, res.Streams)
	})

	t.Run("Subscribe (failure)", func(t *testing.T) {
		channel := fmt.Sprintf("{\"channel\":\"Turbo::StreamsChannel\",\"signed_stream_name\":\"%s\"}", "fake_id")

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
