package pusher

import (
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const identifier = `{"channel":"$pusher","stream":"pucha"}`
const privateIdentifier = `{"channel":"$pusher","stream":"private-party"}`
const presenceIdentifier = `{"channel":"$pusher","stream":"presence-chat"}`

func TestPusherEncode(t *testing.T) {
	coder := NewEncoder()

	t.Run("Reply with event and data", func(t *testing.T) {
		message := map[string]interface{}{
			"event": "custom-event",
			"data":  "custom-data",
		}
		msg := &common.Reply{Identifier: identifier, Message: message}

		expected := `{"event":"custom-event","data":"custom-data","channel":"pucha"}`

		actual, err := coder.Encode(msg)

		require.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("Reply w/o event/data", func(t *testing.T) {
		msg := &common.Reply{Identifier: identifier, Message: "hello"}

		actual, err := coder.Encode(msg)

		require.NoError(t, err)
		assert.Nil(t, actual)
	})

	t.Run("Reply with private channel", func(t *testing.T) {
		message := map[string]interface{}{
			"event": "custom-event",
			"data":  "custom-data",
		}
		msg := &common.Reply{Identifier: privateIdentifier, Message: message}

		expected := `{"event":"custom-event","data":"custom-data","channel":"private-party"}`

		actual, err := coder.Encode(msg)

		require.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("Ping", func(t *testing.T) {
		msg := &common.PingMessage{Type: common.PingType, Message: time.Now().Unix()}

		expected := `{"event":"pusher:ping","data":{}}`

		actual, err := coder.Encode(msg)

		require.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("Pong", func(t *testing.T) {
		msg := &common.Reply{Type: common.PongType}

		expected := `{"event":"pusher:pong"}`

		actual, err := coder.Encode(msg)

		require.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("Disconnect with reconnect", func(t *testing.T) {
		msg := &common.DisconnectMessage{Type: "disconnect", Reason: "server error", Reconnect: true}

		expected := `{"event":"pusher:error","data":{"message":"server error","code":4200}}`

		actual, err := coder.Encode(msg)

		require.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("Disconnect without reconnect", func(t *testing.T) {
		msg := &common.DisconnectMessage{Type: "disconnect", Reason: "unauthorized", Reconnect: false}

		expected := `{"event":"pusher:error","data":{"message":"unauthorized","code":4009}}`

		actual, err := coder.Encode(msg)

		require.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("Welcome", func(t *testing.T) {
		msg := &common.Reply{Type: common.WelcomeType, Sid: "lu2007"}

		expected := `{"event":"pusher:connection_established","data":"{\"socket_id\":\"lu2007\",\"activity_timeout\":30}"}`

		actual, err := coder.Encode(msg)

		require.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("Confirmed", func(t *testing.T) {
		msg := &common.Reply{Type: common.ConfirmedType, Identifier: identifier}

		expected := `{"event":"pusher_internal:subscription_succeeded","data":"{}","channel":"pucha"}`

		actual, err := coder.Encode(msg)

		require.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("Rejected", func(t *testing.T) {
		msg := &common.Reply{Type: common.RejectedType, Identifier: identifier}

		expected := `{"event":"pusher:error","data":{"message":"rejected","code":4009}}`

		actual, err := coder.Encode(msg)

		require.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("Presence join", func(t *testing.T) {
		msg := &common.Reply{
			Type:       common.PresenceType,
			Identifier: presenceIdentifier,
			Message: map[string]interface{}{
				"type": common.PresenceJoinType,
				"id":   "42",
				"info": map[string]interface{}{"name": "Vova"},
			},
		}

		expected := `{"event":"pusher_internal:member_added","data":"{\"user_id\":\"42\",\"user_info\":{\"name\":\"Vova\"}}","channel":"presence-chat"}`

		actual, err := coder.Encode(msg)

		require.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("Presence leave", func(t *testing.T) {
		msg := &common.Reply{
			Type:       common.PresenceType,
			Identifier: presenceIdentifier,
			Message: map[string]interface{}{
				"type": common.PresenceLeaveType,
				"id":   "42",
			},
		}

		expected := `{"event":"pusher_internal:member_removed","data":"{\"user_id\":\"42\"}","channel":"presence-chat"}`

		actual, err := coder.Encode(msg)

		require.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})
}

func TestPusherEncodeTransmission(t *testing.T) {
	coder := NewEncoder()

	t.Run("welcome", func(t *testing.T) {
		msg := `{"type":"welcome","sid":"lu2007"}`
		expected := `{"event":"pusher:connection_established","data":"{\"socket_id\":\"lu2007\",\"activity_timeout\":30}"}`

		actual, err := coder.EncodeTransmission(msg)

		require.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("confirm_subscription", func(t *testing.T) {
		msg := utils.ToJSON(common.Reply{Identifier: identifier, Type: "confirm_subscription"})
		expected := `{"event":"pusher_internal:subscription_succeeded","data":"{}","channel":"pucha"}`

		actual, err := coder.EncodeTransmission(string(msg))

		require.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("confirm_subscription + presence", func(t *testing.T) {
		msg := utils.ToJSON(common.Reply{
			Identifier: identifier, Type: "confirm_subscription",
			Message: `{"presence":{"count":2,"hash":{"xyz":{"name":"jack"}}}}`,
		})
		expected := `{"event":"pusher_internal:subscription_succeeded","data":"{\"presence\":{\"count\":2,\"hash\":{\"xyz\":{\"name\":\"jack\"}}}}","channel":"pucha"}`

		actual, err := coder.EncodeTransmission(string(msg))

		require.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("reject_subscription", func(t *testing.T) {
		msg := utils.ToJSON(common.Reply{Identifier: identifier, Type: "reject_subscription"})
		expected := `{"event":"pusher:error","data":{"message":"rejected","code":4009}}`

		actual, err := coder.EncodeTransmission(string(msg))

		require.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("message with event", func(t *testing.T) {
		message := map[string]interface{}{
			"event": "user-joined",
			"data":  map[string]string{"name": "John"},
		}
		msg := utils.ToJSON(common.Reply{Identifier: identifier, Message: message})
		expected := `{"event":"user-joined","data":{"name":"John"},"channel":"pucha"}`

		actual, err := coder.EncodeTransmission(string(msg))

		require.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})
}

func TestPusherDecode(t *testing.T) {
	coder := NewEncoder()

	t.Run("subscribe", func(t *testing.T) {
		msg := []byte(`{"event":"pusher:subscribe","data":{"auth":"","channel":"pucha"}}`)

		actual, err := coder.Decode(msg)

		require.NoError(t, err)
		assert.Equal(t, "subscribe", actual.Command)
		assert.Equal(t, identifier, actual.Identifier)

		data, ok := actual.Data.(*PusherSubscriptionData)
		assert.True(t, ok)
		assert.Equal(t, "", data.Auth)
	})

	t.Run("subscribe to private", func(t *testing.T) {
		msg := []byte(`{"event":"pusher:subscribe","data":{"auth":"signed-private-party","channel":"private-party"}}`)

		actual, err := coder.Decode(msg)

		require.NoError(t, err)
		assert.Equal(t, "subscribe", actual.Command)
		assert.Equal(t, privateIdentifier, actual.Identifier)

		data, ok := actual.Data.(*PusherSubscriptionData)
		assert.True(t, ok)
		assert.Equal(t, "signed-private-party", data.Auth)
	})

	t.Run("unsubscribe", func(t *testing.T) {
		msg := []byte(`{"event":"pusher:unsubscribe","data":{"channel":"pucha"}}`)

		actual, err := coder.Decode(msg)

		require.NoError(t, err)
		assert.Equal(t, "unsubscribe", actual.Command)
		assert.Equal(t, identifier, actual.Identifier)
	})

	t.Run("ping", func(t *testing.T) {
		msg := []byte(`{"event":"pusher:ping","data":{}}`)

		actual, err := coder.Decode(msg)

		require.NoError(t, err)
		assert.Equal(t, "ping", actual.Command)
	})

	t.Run("pong", func(t *testing.T) {
		msg := []byte(`{"event":"pusher:pong"}`)

		actual, err := coder.Decode(msg)

		require.NoError(t, err)
		assert.Equal(t, "pong", actual.Command)
	})

	t.Run("client event", func(t *testing.T) {
		msg := []byte(`{"event":"client-message","data":{"text":"hello"},"channel":"private-party"}`)
		actual, err := coder.Decode(msg)

		payload := &PusherClientEvent{
			Event:   "client-message",
			Channel: "private-party",
			Data: map[string]interface{}{
				"text": "hello",
			},
		}

		assert.NoError(t, err)
		require.NotNil(t, actual)
		assert.Equal(t, "whisper", actual.Command)
		assert.Equal(t, privateIdentifier, actual.Identifier)
		assert.Equal(t, payload, actual.Data)
	})

	t.Run("custom event", func(t *testing.T) {
		msg := []byte(`{"event":"custom-message","data":{"channel":"test-channel","text":"hello"}}`)

		actual, err := coder.Decode(msg)

		assert.Error(t, err)
		assert.Nil(t, actual)
	})
}
