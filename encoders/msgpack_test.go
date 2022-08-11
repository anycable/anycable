package encoders

import (
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/ws"
	"github.com/stretchr/testify/assert"
	"github.com/vmihailenco/msgpack/v5"
)

type testMessage struct {
	Type       string      `msgpack:"type,omitempty"`
	Identifier string      `msgpack:"identifier,omitempty"`
	Message    interface{} `msgpack:"message,omitempty"`
	Command    string      `msgpack:"command,omitempty"`
	Data       string      `msgpack:"data,omitempty"`
}

func TestMsgpackEncoder(t *testing.T) {
	coder := Msgpack{}

	t.Run(".Encode", func(t *testing.T) {
		msg := &common.Reply{Type: "test", Identifier: "test_channel", Message: "hello"}
		expectedMsg := &testMessage{Type: "test", Identifier: "test_channel", Message: "hello"}

		expected, _ := msgpack.Marshal(&expectedMsg)

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
		assert.Equal(t, ws.BinaryFrame, actual.FrameType)
	})

	t.Run(".EncodeTransmission", func(t *testing.T) {
		msg := "{\"type\":\"test\",\"identifier\":\"test_channel\",\"message\":\"hello\"}"
		command := testMessage{Type: "test", Identifier: "test_channel", Message: "hello"}
		expected, _ := msgpack.Marshal(&command)

		actual, err := coder.EncodeTransmission(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
		assert.Equal(t, ws.BinaryFrame, actual.FrameType)
	})

	t.Run(".Decode", func(t *testing.T) {
		command := testMessage{Command: "test", Identifier: "test_channel", Data: "hello"}
		msg, _ := msgpack.Marshal(&command)

		actual, err := coder.Decode(msg)

		assert.NoError(t, err)
		assert.Equal(t, actual.Command, "test")
		assert.Equal(t, actual.Identifier, "test_channel")
		assert.Equal(t, actual.Data, "hello")
	})
}
