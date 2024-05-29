package sse

import (
	"fmt"
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/stretchr/testify/assert"
)

func TestEncoder_Encode(t *testing.T) {
	coder := Encoder{}

	t.Run("without type", func(t *testing.T) {
		msg := &common.Reply{Identifier: "test_channel", Message: "hello"}
		expected := `data: {"identifier":"test_channel","message":"hello"}`

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("without type + unwrap data", func(t *testing.T) {
		getCoder := Encoder{UnwrapData: true}
		msg := &common.Reply{Identifier: "test_channel", Message: "hello"}
		expected := `data: hello`

		actual, err := getCoder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("without type + unwrap data + raw data", func(t *testing.T) {
		getCoder := Encoder{UnwrapData: true, RawData: true}
		msg := &common.Reply{Identifier: "test_channel", Message: "hello"}
		expected := `data: hello`

		actual, err := getCoder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("with type", func(t *testing.T) {
		rawCoder := Encoder{RawData: true}
		msg := &common.Reply{Type: "test", Identifier: "test_channel", Message: "hello"}

		actual, err := rawCoder.Encode(msg)

		assert.NoError(t, err)
		assert.Nil(t, actual)
	})

	t.Run("with type + raw data", func(t *testing.T) {
		msg := &common.Reply{Type: "test", Identifier: "test_channel", Message: "hello"}
		expected := "event: test\n" +
			`data: {"type":"test","identifier":"test_channel","message":"hello"}`

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("with offset and stream id", func(t *testing.T) {
		msg := &common.Reply{Type: "test", Identifier: "test_channel", Message: "hello", StreamID: "stream-test", Epoch: "bc320", Offset: 321}
		expected := "event: test\n" +
			`data: {"type":"test","identifier":"test_channel","message":"hello","stream_id":"stream-test","epoch":"bc320","offset":321}` + "\n" +
			"id: 321/bc320/stream-test"

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("when disconnect with reconnect=true", func(t *testing.T) {
		msg := &common.Reply{Type: "disconnect", Reason: "unknown", Reconnect: true}
		expected := "event: disconnect\n" +
			`data: {"type":"disconnect","reason":"unknown","reconnect":true}`

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("when disconnect with reconnect=false", func(t *testing.T) {
		msg := &common.DisconnectMessage{Type: "disconnect", Reason: "unknown", Reconnect: false}
		expected := "event: disconnect\n" +
			`data: {"type":"disconnect","reason":"unknown","reconnect":false}` + "\n" +
			"retry: " + fmt.Sprintf("%d", retryNoReconnect)

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})
}

func TestEncoder_EncodeTransmission(t *testing.T) {
	coder := Encoder{}

	t.Run("w/type", func(t *testing.T) {
		msg := "{\"type\":\"test\",\"identifier\":\"test_channel\",\"message\":\"hello\"}"
		expected := []byte(fmt.Sprintf("event: test\ndata: %s", msg))

		actual, err := coder.EncodeTransmission(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
	})

	t.Run("w/o type", func(t *testing.T) {
		msg := "{\"message\":\"hello\"}"
		expected := []byte(fmt.Sprintf("data: %s", msg))

		actual, err := coder.EncodeTransmission(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
	})
}

func TestEncoder_Decode(t *testing.T) {
	coder := Encoder{}

	msg := []byte("{\"command\":\"test\",\"identifier\":\"test_channel\",\"data\":\"hello\"}")

	actual, err := coder.Decode(msg)

	assert.Error(t, err)
	assert.Nil(t, actual)
}
