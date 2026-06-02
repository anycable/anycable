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

	t.Run("without type + unwrap data + multiline payload", func(t *testing.T) {
		getCoder := Encoder{UnwrapData: true}
		// Multi-line payloads (e.g. Turbo Stream HTML) must be split across
		// multiple `data:` fields, otherwise EventSource truncates at the first
		// newline. The client rejoins the fields with "\n".
		msg := &common.Reply{Identifier: "test_channel", Message: "<turbo-stream>\n<template>\nhi\n</template>\n</turbo-stream>"}
		expected := "data: <turbo-stream>\n" +
			"data: <template>\n" +
			"data: hi\n" +
			"data: </template>\n" +
			"data: </turbo-stream>"

		actual, err := getCoder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("without type + unwrap data + CRLF and CR terminators", func(t *testing.T) {
		getCoder := Encoder{UnwrapData: true}
		// LF, CR and CRLF are all SSE line terminators and must each start a
		// new data: field.
		msg := &common.Reply{Identifier: "test_channel", Message: "a\r\nb\rc\nd"}
		expected := "data: a\n" +
			"data: b\n" +
			"data: c\n" +
			"data: d"

		actual, err := getCoder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, string(actual.Payload))
	})

	t.Run("with type + multiline payload preserves event and id", func(t *testing.T) {
		msg := &common.Reply{Type: "test", Identifier: "test_channel", Message: "a\nb", StreamID: "stream-test", Epoch: "bc320", Offset: 321}
		expected := "event: test\n" +
			`data: {"type":"test","identifier":"test_channel","message":"a\nb","stream_id":"stream-test","epoch":"bc320","offset":321}` + "\n" +
			"id: 321/bc320/stream-test"

		actual, err := coder.Encode(msg)

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
