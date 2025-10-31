package encoders

import (
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/ws"
	"github.com/golang/protobuf/proto" // nolint:staticcheck
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"

	pb "github.com/anycable/anycable-go/ac_protos"
)

func TestProtobufEncoder(t *testing.T) {
	coder := Protobuf{}

	t.Run(".Encode", func(t *testing.T) {
		payload, _ := msgpack.Marshal("hello")
		msg := &pb.Reply{Identifier: "test_channel", Message: payload}

		expected, _ := proto.Marshal(msg)

		actual, err := coder.Encode(&common.Reply{Identifier: "test_channel", Message: "hello"})

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
		assert.Equal(t, ws.BinaryFrame, actual.FrameType)
	})

	t.Run(".Encode history ack", func(t *testing.T) {
		msg := &pb.Reply{Identifier: "test_channel", Type: pb.Type_confirm_history}

		expected, _ := proto.Marshal(msg)

		actual, err := coder.Encode(&common.Reply{Identifier: "test_channel", Type: "confirm_history"})

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
		assert.Equal(t, ws.BinaryFrame, actual.FrameType)
	})

	t.Run(".EncodeTransmission confirm_subscription", func(t *testing.T) {
		msg := "{\"type\":\"confirm_subscription\",\"identifier\":\"test_channel\",\"message\":\"hello\"}"
		payload, _ := msgpack.Marshal("hello")
		command := &pb.Reply{Type: pb.Type_confirm_subscription, Identifier: "test_channel", Message: payload}
		expected, _ := proto.Marshal(command)

		actual, err := coder.EncodeTransmission(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
		assert.Equal(t, ws.BinaryFrame, actual.FrameType)
	})

	t.Run(".EncodeTransmission welcome", func(t *testing.T) {
		msg := "{\"type\":\"welcome\"}"
		command := &pb.Reply{Type: pb.Type_welcome}
		expected, _ := proto.Marshal(command)

		actual, err := coder.EncodeTransmission(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
		assert.Equal(t, ws.BinaryFrame, actual.FrameType)
	})

	t.Run(".EncodeTransmission message", func(t *testing.T) {
		msg := "{\"identifier\":\"test_channel\",\"message\":\"hello\"}"
		payload, _ := msgpack.Marshal("hello")
		command := &pb.Reply{Type: pb.Type_no_type, Identifier: "test_channel", Message: payload}
		expected, _ := proto.Marshal(command)

		actual, err := coder.EncodeTransmission(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
		assert.Equal(t, ws.BinaryFrame, actual.FrameType)
	})

	t.Run(".EncodeTransmission disconnect", func(t *testing.T) {
		msg := "{\"type\":\"disconnect\",\"reason\":\"unauthorized\",\"reconnect\":false}"
		command := &pb.Reply{Type: pb.Type_disconnect, Reconnect: false, Reason: "unauthorized"}
		expected, _ := proto.Marshal(command)

		actual, err := coder.EncodeTransmission(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
		assert.Equal(t, ws.BinaryFrame, actual.FrameType)
	})

	t.Run(".Decode", func(t *testing.T) {
		command := &pb.Message{Command: pb.Command_message, Identifier: "test_channel", Data: "hello"}
		msg, _ := proto.Marshal(command)

		actual, err := coder.Decode(msg)

		assert.NoError(t, err)
		assert.Equal(t, actual.Command, "message")
		assert.Equal(t, actual.Identifier, "test_channel")
		assert.Equal(t, actual.Data, "hello")
	})

	t.Run(".Decode w/ History", func(t *testing.T) {
		command := &pb.Message{
			Command:    pb.Command_history,
			Identifier: "test_channel",
			Data:       "hello",
			History: &pb.HistoryRequest{
				Since:   333,
				Streams: map[string]*pb.StreamHistoryRequest{"test_stream": {Epoch: "bc", Offset: 32}},
			},
		}

		msg, _ := proto.Marshal(command)

		actual, err := coder.Decode(msg)

		require.NoError(t, err)
		assert.Equal(t, "history", actual.Command)
		assert.Equal(t, "test_channel", actual.Identifier)
		assert.Equal(t, "hello", actual.Data)
		assert.EqualValues(t, 333, actual.History.Since)
		assert.Equal(t, "bc", actual.History.Streams["test_stream"].Epoch)
		assert.EqualValues(t, 32, actual.History.Streams["test_stream"].Offset)
	})

	t.Run(".Decode whisper", func(t *testing.T) {
		command := &pb.Message{Command: pb.Command_whisper, Identifier: "test_channel", Data: `{"msg":"hello"}`}
		msg, _ := proto.Marshal(command)

		actual, err := coder.Decode(msg)

		assert.NoError(t, err)
		assert.Equal(t, actual.Command, "whisper")
		assert.Equal(t, actual.Identifier, "test_channel")
		assert.Equal(t, actual.Data, `{"msg":"hello"}`)
	})

}
