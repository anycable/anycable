package rails

import (
	"fmt"
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCableReadyController(t *testing.T) {
	key := "s3Krit"
	// CableReady::Config.instance.verifier_key = 's3Krit'
	// CableReady.signed_stream_verifier.generate("stream:2021")
	stream := "InN0cmVhbToyMDIxIg==--44f6315dd9faefe713ef5685e114413c1afe8759197a0fc39b15cee75769417e"

	env := common.NewSessionEnv("ws://demo.anycable.io/cable", &map[string]string{"cookie": "val=1;"})
	subject := NewCableReadyController(key)

	t.Run("Subscribe (success)", func(t *testing.T) {
		channel := fmt.Sprintf("{\"channel\":\"CableReady::Stream\",\"identifier\":\"%s\"}", stream)

		res, err := subject.Subscribe("42", env, "name=jack", channel)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{common.ConfirmationMessage(channel)}, res.Transmissions)
		assert.Equal(t, []string{"stream:2021"}, res.Streams)
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

func TestCableReadyControllerWithClearText(t *testing.T) {
	env := common.NewSessionEnv("ws://demo.anycable.io/cable", &map[string]string{"cookie": "val=1;"})
	subject := NewCableReadyController("")

	t.Run("Subscribe (success)", func(t *testing.T) {
		channel := "{\"channel\":\"CableReady::Stream\",\"identifier\":\"stream:2023\"}"

		res, err := subject.Subscribe("42", env, "name=jack", channel)

		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, common.SUCCESS, res.Status)
		assert.Equal(t, []string{common.ConfirmationMessage(channel)}, res.Transmissions)
		assert.Equal(t, []string{"stream:2023"}, res.Streams)
		assert.Equal(t, -1, res.DisconnectInterest)
	})
}
