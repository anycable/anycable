package encoders

import (
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/ws"
	"github.com/golang/protobuf/proto" // nolint:staticcheck
	"github.com/stretchr/testify/assert"
	"github.com/vmihailenco/msgpack/v5"

	pb "github.com/anycable/anycable-go/ac_protos"
)

func TestProtobufEncoder(t *testing.T) {
	coder := Protobuf{}

	t.Run(".Encode", func(t *testing.T) {
		payload, _ := msgpack.Marshal("hello")
		msg := &pb.Message{Identifier: "test_channel", Message: payload}

		expected, _ := proto.Marshal(msg)

		actual, err := coder.Encode(&common.Reply{Identifier: "test_channel", Message: "hello"})

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
		assert.Equal(t, ws.BinaryFrame, actual.FrameType)
	})

	t.Run(".EncodeTransmission confirm_subscription", func(t *testing.T) {
		msg := "{\"type\":\"confirm_subscription\",\"identifier\":\"test_channel\",\"message\":\"hello\"}"
		payload, _ := msgpack.Marshal("hello")
		command := &pb.Message{Type: pb.Type_confirm_subscription, Identifier: "test_channel", Message: payload}
		expected, _ := proto.Marshal(command)

		actual, err := coder.EncodeTransmission(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
		assert.Equal(t, ws.BinaryFrame, actual.FrameType)
	})

	t.Run(".EncodeTransmission welcome", func(t *testing.T) {
		msg := "{\"type\":\"welcome\"}"
		command := &pb.Message{Type: pb.Type_welcome}
		expected, _ := proto.Marshal(command)

		actual, err := coder.EncodeTransmission(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
		assert.Equal(t, ws.BinaryFrame, actual.FrameType)
	})

	t.Run(".EncodeTransmission message", func(t *testing.T) {
		msg := "{\"identifier\":\"test_channel\",\"message\":\"hello\"}"
		payload, _ := msgpack.Marshal("hello")
		command := &pb.Message{Type: pb.Type_no_type, Identifier: "test_channel", Message: payload}
		expected, _ := proto.Marshal(command)

		actual, err := coder.EncodeTransmission(msg)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual.Payload)
		assert.Equal(t, ws.BinaryFrame, actual.FrameType)
	})

	t.Run(".EncodeTransmission disconnect", func(t *testing.T) {
		msg := "{\"type\":\"disconnect\",\"reason\":\"unauthorized\",\"reconnect\":false}"
		command := &pb.Message{Type: pb.Type_disconnect, Reconnect: false, Reason: "unauthorized"}
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
}
