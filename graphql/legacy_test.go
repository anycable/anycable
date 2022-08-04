package graphql

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/utils"
	"github.com/stretchr/testify/assert"
)

const legacyIdentifier = "{\"channel\":\"GraphqlChannel\",\"channelId\":\"abc2021\"}"

func TestLegacyGraphQLEncode(t *testing.T) {
	coder := LegacyEncoder{}

	t.Run("Reply", func(t *testing.T) {
		msg := &common.Reply{Identifier: legacyIdentifier, Message: "hello"}

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

		expected := "{\"type\":\"connection_error\"}"

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("Unsubscribed", func(t *testing.T) {
		msg := &common.Reply{Type: common.UnsubscribedType, Identifier: legacyIdentifier}

		expected := "{\"id\":\"abc2021\",\"type\":\"complete\"}"

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})
}

func TestLegacyGraphQLEncodeTransmission(t *testing.T) {
	coder := LegacyEncoder{}

	t.Run("welcome", func(t *testing.T) {
		msg := "{\"type\":\"welcome\"}"
		expected := "{\"type\":\"connection_ack\"}"

		actual, err := coder.EncodeTransmission(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("confirm_subscription", func(t *testing.T) {
		msg := utils.ToJSON(common.Reply{Identifier: legacyIdentifier, Type: "confirm_subscription"})

		actual, err := coder.EncodeTransmission(string(msg))

		assert.NoError(t, err)
		assert.Nil(t, actual)
	})

	t.Run("reject_subscription", func(t *testing.T) {
		msg := utils.ToJSON(common.Reply{Identifier: legacyIdentifier, Type: "reject_subscription"})
		expected := "{\"id\":\"abc2021\",\"type\":\"error\"}"

		actual, err := coder.EncodeTransmission(string(msg))

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("unsubscribed", func(t *testing.T) {
		msg := utils.ToJSON(common.Reply{Identifier: legacyIdentifier, Type: "unsubscribed"})
		expected := "{\"id\":\"abc2021\",\"type\":\"complete\"}"

		actual, err := coder.EncodeTransmission(string(msg))

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("message", func(t *testing.T) {
		msg := utils.ToJSON(common.Reply{Identifier: legacyIdentifier, Message: "payload"})
		expected := "{\"id\":\"abc2021\",\"type\":\"data\",\"payload\":\"payload\"}"

		actual, err := coder.EncodeTransmission(string(msg))

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("message with result", func(t *testing.T) {
		var result interface{}

		json.Unmarshal([]byte("{\"result\":\"payload\"}"), &result) // nolint:errcheck

		msg := utils.ToJSON(common.Reply{Identifier: legacyIdentifier, Message: result})
		expected := "{\"id\":\"abc2021\",\"type\":\"data\",\"payload\":\"payload\"}"

		actual, err := coder.EncodeTransmission(string(msg))

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})
}

func TestLegacyGraphQLDecode(t *testing.T) {
	coder := LegacyEncoder{}

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
