package graphql

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/utils"
	"github.com/stretchr/testify/assert"
)

const identifier = "{\"channel\":\"GraphqlChannel\",\"channelId\":\"abc2022\"}"

func TestGraphQLEncode(t *testing.T) {
	coder := Encoder{}

	t.Run("Reply", func(t *testing.T) {
		msg := &common.Reply{Identifier: identifier, Message: "hello"}

		expected := "{\"id\":\"abc2022\",\"type\":\"next\",\"payload\":\"hello\"}"

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("Ping", func(t *testing.T) {
		msg := &common.PingMessage{Type: "ping", Message: time.Now().Unix()}

		expected := "{\"type\":\"ping\"}"

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("Pong", func(t *testing.T) {
		msg := &common.Reply{Type: "pong"}

		expected := "{\"type\":\"pong\"}"

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("Disconnect", func(t *testing.T) {
		msg := &common.DisconnectMessage{Type: "disconnect", Reason: "unauthorized", Reconnect: false}

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Nil(t, actual)
	})

	t.Run("Unsubscribed", func(t *testing.T) {
		msg := &common.Reply{Type: common.UnsubscribedType, Identifier: identifier}

		expected := "{\"id\":\"abc2022\",\"type\":\"complete\"}"

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
		msg := utils.ToJSON(common.Reply{Identifier: identifier, Type: "confirm_subscription"})

		actual, err := coder.EncodeTransmission(string(msg))

		assert.NoError(t, err)
		assert.Nil(t, actual)
	})

	t.Run("reject_subscription", func(t *testing.T) {
		msg := utils.ToJSON(common.Reply{Identifier: identifier, Type: "reject_subscription"})
		expected := "{\"id\":\"abc2022\",\"type\":\"error\"}"

		actual, err := coder.EncodeTransmission(string(msg))

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("unsubscribed", func(t *testing.T) {
		msg := utils.ToJSON(common.Reply{Identifier: identifier, Type: "unsubscribed"})
		expected := "{\"id\":\"abc2022\",\"type\":\"complete\"}"

		actual, err := coder.EncodeTransmission(string(msg))

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("message", func(t *testing.T) {
		msg := utils.ToJSON(common.Reply{Identifier: identifier, Message: "payload"})
		expected := "{\"id\":\"abc2022\",\"type\":\"next\",\"payload\":\"payload\"}"

		actual, err := coder.EncodeTransmission(string(msg))

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("message with result", func(t *testing.T) {
		var result interface{}

		json.Unmarshal([]byte("{\"result\":\"payload\"}"), &result) // nolint:errcheck

		msg := utils.ToJSON(common.Reply{Identifier: identifier, Message: result})
		expected := "{\"id\":\"abc2022\",\"type\":\"next\",\"payload\":\"payload\"}"

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

	t.Run("init with payload", func(t *testing.T) {
		msg := []byte("{\"type\":\"connection_init\",\"payload\":{\"token\":\"secret\"}}")

		actual, err := coder.Decode(msg)

		assert.NoError(t, err)
		assert.Equal(t, "connection_init", actual.Command)
		assert.Equal(t, "{\"token\":\"secret\"}", actual.Data)
	})

	t.Run("subscribe", func(t *testing.T) {
		msg := []byte("{\"type\":\"subscribe\",\"id\":\"abc2022\",\"payload\":{\"query\":\"Post { id }\"}}")

		actual, err := coder.Decode(msg)

		assert.NoError(t, err)
		assert.Equal(t, "subscribe", actual.Command)
		assert.Equal(t, "abc2022", actual.Identifier)
		assert.Equal(t, "{\"query\":\"Post { id }\"}", actual.Data)
	})

	t.Run("complete", func(t *testing.T) {
		msg := []byte("{\"type\":\"complete\",\"id\":\"abc2022\"}")

		actual, err := coder.Decode(msg)

		assert.NoError(t, err)
		assert.Equal(t, "complete", actual.Command)
		assert.Equal(t, "abc2022", actual.Identifier)
	})
}
