package apollo

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/stretchr/testify/assert"
)

const identifier = "{\"channel\":\"GraphqlChannel\",\"channelId\":\"abc2021\"}"

func TestGraphQLEncode(t *testing.T) {
	coder := Encoder{}

	t.Run("Reply", func(t *testing.T) {
		msg := &common.Reply{Identifier: identifier, Message: "hello"}

		expected := "{\"id\":\"abc2021\",\"type\":\"data\",\"payload\":\"hello\"}"

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("Ping", func(t *testing.T) {
		msg := &common.PingMessage{Type: "ping", Message: time.Now().Unix()}

		expected := "{\"type\":\"ka\"}"

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("Disconnect", func(t *testing.T) {
		msg := &common.DisconnectMessage{Type: "disconnect", Reason: "unauthorized", Reconnect: false}

		expected := "{\"type\":\"conn_err\"}"

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})
}

func TestGraphQLEncodeTransmission(t *testing.T) {
	coder := Encoder{}

	t.Run("welcome", func(t *testing.T) {
		msg := "{\"type\":\"welcome\"}"
		expected := "{\"type\":\"connection_ack\"}"

		actual, err := coder.EncodeTransmission(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("confirm_subscription", func(t *testing.T) {
		msg := toJSON(common.Reply{Identifier: identifier, Type: "confirm_subscription"})

		actual, err := coder.EncodeTransmission(string(msg))

		assert.NoError(t, err)
		assert.Nil(t, actual)
	})

	t.Run("reject_subscription", func(t *testing.T) {
		msg := toJSON(common.Reply{Identifier: identifier, Type: "reject_subscription"})
		expected := "{\"id\":\"abc2021\",\"type\":\"error\"}"

		actual, err := coder.EncodeTransmission(string(msg))

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("unsubscribed", func(t *testing.T) {
		msg := toJSON(common.Reply{Identifier: identifier, Type: "unsubscribed"})
		expected := "{\"id\":\"abc2021\",\"type\":\"stop\"}"

		actual, err := coder.EncodeTransmission(string(msg))

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("message", func(t *testing.T) {
		msg := toJSON(common.Reply{Identifier: identifier, Message: "payload"})
		expected := "{\"id\":\"abc2021\",\"type\":\"data\",\"payload\":\"payload\"}"

		actual, err := coder.EncodeTransmission(string(msg))

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})
}

func TestGraphQLDecode(t *testing.T) {
	coder := Encoder{}

	t.Run("init", func(t *testing.T) {
		msg := []byte("{\"type\":\"connection_init\"}")

		actual, err := coder.Decode(msg)

		assert.NoError(t, err)
		assert.Equal(t, "connection_init", actual.Command)
	})

	t.Run("start", func(t *testing.T) {
		msg := []byte("{\"type\":\"start\",\"id\":\"abc2021\",\"payload\":{\"query\":\"Post { id }\"}}")

		actual, err := coder.Decode(msg)

		assert.NoError(t, err)
		assert.Equal(t, "start", actual.Command)
		assert.Equal(t, "abc2021", actual.Identifier)
		assert.Equal(t, "{\"query\":\"Post { id }\"}", actual.Data)
	})

	t.Run("stop", func(t *testing.T) {
		msg := []byte("{\"type\":\"stop\",\"id\":\"abc2021\"}")

		actual, err := coder.Decode(msg)

		assert.NoError(t, err)
		assert.Equal(t, "stop", actual.Command)
		assert.Equal(t, "abc2021", actual.Identifier)
	})

	t.Run("terminate", func(t *testing.T) {
		msg := []byte("{\"type\":\"connection_terminate\"}")

		actual, err := coder.Decode(msg)

		assert.NoError(t, err)
		assert.Equal(t, "connection_terminate", actual.Command)
	})
}

func toJSON(msg common.Reply) []byte {
	b, err := json.Marshal(&msg)
	if err != nil {
		panic("Failed to build JSON 😲")
	}

	return b
}
