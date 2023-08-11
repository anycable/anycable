package encoders

import (
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
)

type testReply struct {
	Type        string      `msgpack:"type,omitempty"`
	Identifier  string      `msgpack:"identifier,omitempty"`
	Message     interface{} `msgpack:"message,omitempty"`
	Reason      string      `msgpack:"reason,omitempty"`
	Reconnect   bool        `msgpack:"reconnect,omitempty"`
	StreamID    string      `msgpack:"stream,omitempty"`
	Epoch       string      `msgpack:"epoch,omitempty"`
	Offset      uint64      `msgpack:"offset,omitempty"`
	Sid         string      `msgpack:"sid,omitempty"`
	Restored    bool        `msgpack:"restored,omitempty"`
	RestoredIDs []string    `msgpack:"restored_ids,omitempty"`
}

type testHistoryPosition struct {
	Epoch  string `msgpack:"epoch,omitempty"`
	Offset uint64 `msgpack:"offset,omitempty"`
}

type testHistory struct {
	Since   int64                          `msgpack:"since,omitempty"`
	Streams map[string]testHistoryPosition `msgpack:"streams,omitempty"`
}

type testMessage struct {
	Identifier string      `msgpack:"identifier,omitempty"`
	Data       string      `msgpack:"data,omitempty"`
	Command    string      `msgpack:"command,omitempty"`
	History    testHistory `msgpack:"history,omitempty"`
}

func TestMsgpackEncoder(t *testing.T) {
	coder := Msgpack{}

	t.Run(".Encode", func(t *testing.T) {
		msg := &common.Reply{Type: "test", Identifier: "test_channel", Message: "hello"}
		expectedMsg := &testReply{Type: "test", Identifier: "test_channel", Message: "hello"}

		expected, _ := msgpack.Marshal(&expectedMsg)

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
		assert.Equal(t, ws.BinaryFrame, actual.FrameType)
	})

	t.Run(".Encode w/ stream meta", func(t *testing.T) {
		msg := &common.Reply{Type: "test", Identifier: "test_channel", Message: "hello", Epoch: "123", Offset: 33}
		expectedMsg := &testReply{Type: "test", Identifier: "test_channel", Message: "hello", Epoch: "123", Offset: 33}

		expected, _ := msgpack.Marshal(&expectedMsg)

		actual, err := coder.Encode(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
		assert.Equal(t, ws.BinaryFrame, actual.FrameType)
	})

	t.Run(".EncodeTransmission", func(t *testing.T) {
		msg := "{\"type\":\"test\",\"identifier\":\"test_channel\",\"message\":\"hello\"}"
		command := testReply{Type: "test", Identifier: "test_channel", Message: "hello"}
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

	t.Run(".Decode w/ History", func(t *testing.T) {
		command := testMessage{
			Command:    "history",
			Identifier: "test_channel",
			History: testHistory{
				Streams: map[string]testHistoryPosition{
					"test_stream": {Epoch: "123", Offset: 33},
				},
			},
		}

		msg, _ := msgpack.Marshal(&command)

		actual, err := coder.Decode(msg)

		require.NoError(t, err)
		assert.Equal(t, actual.Command, "history")
		assert.Equal(t, actual.Identifier, "test_channel")
		assert.Equal(t, actual.History.Streams["test_stream"].Epoch, "123")
		assert.EqualValues(t, actual.History.Streams["test_stream"].Offset, 33)
	})
}
