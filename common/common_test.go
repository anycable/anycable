package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSessionStateMergeConnectionState(t *testing.T) {
	env := NewSessionEnv("", nil)
	(*env.ConnectionState)["a"] = "old"

	t.Run("when adding and rewriting", func(t *testing.T) {
		env.MergeConnectionState(&map[string]string{"a": "new", "b": "super new"})

		assert.Len(t, *env.ConnectionState, 2)
		assert.Equal(t, "new", (*env.ConnectionState)["a"])
		assert.Equal(t, "super new", (*env.ConnectionState)["b"])
	})
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
