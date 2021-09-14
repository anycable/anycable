package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvMergeConnectionState(t *testing.T) {
	env := NewSessionEnv("", nil)
	(*env.ConnectionState)["a"] = "old"

	t.Run("when adding and rewriting", func(t *testing.T) {
		env.MergeConnectionState(&map[string]string{"a": "new", "b": "super new"})

		assert.Len(t, *env.ConnectionState, 2)
		assert.Equal(t, "new", (*env.ConnectionState)["a"])
		assert.Equal(t, "super new", (*env.ConnectionState)["b"])
	})
}

func TestEnvMergeChannelState(t *testing.T) {
	env := NewSessionEnv("", nil)
	(*env.ChannelStates)["id"] = map[string]string{"a": "old"}

	t.Run("when adding and rewriting", func(t *testing.T) {
		env.MergeChannelState("id", &map[string]string{"a": "new", "b": "super new"})

		assert.Len(t, (*env.ChannelStates)["id"], 2)
		assert.Equal(t, "new", (*env.ChannelStates)["id"]["a"])
		assert.Equal(t, "super new", (*env.ChannelStates)["id"]["b"])
	})

	t.Run("when new channel", func(t *testing.T) {
		env.MergeChannelState("id2", &map[string]string{"a": "new"})

		assert.Len(t, (*env.ChannelStates)["id2"], 1)
		assert.Equal(t, "new", (*env.ChannelStates)["id2"]["a"])
	})
}

func TestEnvGetChannelStateField(t *testing.T) {
	env := NewSessionEnv("", nil)
	(*env.ChannelStates)["id"] = map[string]string{"a": "old"}

	assert.Equal(t, "old", env.GetChannelStateField("id", "a"))
	assert.Equal(t, "", env.GetChannelStateField("no_id", "a"))
	assert.Equal(t, "", env.GetChannelStateField("id", "no_field"))
}

func TestEnvGetConnectionStateField(t *testing.T) {
	env := NewSessionEnv("", nil)
	(*env.ConnectionState)["a"] = "old"

	assert.Equal(t, "old", env.GetConnectionStateField("a"))
	assert.Equal(t, "", env.GetConnectionStateField("no_a"))
}

func TestPubSubMessageFromJSON(t *testing.T) {
	t.Run("Remote disconnect message", func(t *testing.T) {
		msg := []byte("{\"command\":\"disconnect\",\"payload\":{\"identifier\":\"14\",\"reconnect\":false}}")

		result, err := PubSubMessageFromJSON(msg)
		assert.Nil(t, err)

		casted := result.(RemoteDisconnectMessage)
		assert.Equal(t, "14", casted.Identifier)
		assert.Equal(t, false, casted.Reconnect)
	})

	t.Run("Broadcast message", func(t *testing.T) {
		msg := []byte("{\"stream\":\"bread-test\",\"data\":\"test\"}")

		result, err := PubSubMessageFromJSON(msg)
		assert.Nil(t, err)

		casted := result.(StreamMessage)
		assert.Equal(t, "bread-test", casted.Stream)
		assert.Equal(t, "test", casted.Data)
	})
}

func TestConfirmationMessage(t *testing.T) {
	assert.Equal(t, "{\"type\":\"confirm_subscription\",\"identifier\":\"test_channel\"}", ConfirmationMessage("test_channel"))
}

func TestRejectionMessage(t *testing.T) {
	assert.Equal(t, "{\"type\":\"reject_subscription\",\"identifier\":\"test_channel\"}", RejectionMessage("test_channel"))
}
