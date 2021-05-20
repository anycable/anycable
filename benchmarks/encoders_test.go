package encoders

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/anycable/anycable-go/apollo"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/golang/protobuf/proto" // nolint:staticcheck
	"github.com/stretchr/testify/assert"
	"github.com/vmihailenco/msgpack/v5"

	pb "github.com/anycable/anycable-go/ac_protos"
)

var (
	identifier     = "{\"channel\":\"test_channel\",\"channelId\":\"23\"}"
	longIdentifier = fmt.Sprintf("{\"channel\":\"%s\",\"channelId\":\"%s\"}", strings.Repeat("test_channel", 10), strings.Repeat("123", 10))
)

type message struct {
	Type       string      `json:"type,omitempty" msgpack:"type,omitempty"`
	Identifier string      `json:"identifier,omitempty" msgpack:"identifier,omitempty"`
	Message    interface{} `json:"message,omitempty" msgpack:"message,omitempty"`
	Command    string      `json:"command,omitempty" msgpack:"command,omitempty"`
	Data       string      `json:"data,omitempty" msgpack:"data,omitempty"`
}

func BenchmarkEncodersDecode(b *testing.B) {
	baseCmd := message{Command: "message", Identifier: identifier, Data: "hello world"}
	baseMsg, _ := json.Marshal(&baseCmd)

	longCmd := message{Command: "message", Identifier: longIdentifier, Message: baseMsg}
	longMsg, _ := json.Marshal(&longCmd)

	baseApolloMsg := []byte("{\"type\":\"start\",\"id\":\"abc2021\",\"payload\":{\"query\":\"Post { id }\"}}")
	longApolloMsg := []byte(fmt.Sprintf("{\"type\":\"start\",\"id\":\"%s\",\"payload\":{\"query\":%s}}", strings.Repeat("abcd_efjhkl", 10), baseApolloMsg))

	baseMsgpack, _ := msgpack.Marshal(&baseCmd)
	longMsgpack, _ := msgpack.Marshal(&longCmd)

	baseProtobuf, _ := proto.Marshal(&pb.Message{Command: pb.Command_message, Identifier: identifier, Data: "hello world"})
	longProtobuf, _ := proto.Marshal(&pb.Message{Command: pb.Command_message, Identifier: longIdentifier, Message: baseMsgpack})

	configs := []struct {
		title   string
		input   []byte
		encoder encoders.Encoder
	}{
		{"JSON base", baseMsg, encoders.JSON{}},
		{"JSON long", longMsg, encoders.JSON{}},
		{"Apollo base", baseApolloMsg, apollo.Encoder{}},
		{"Apollo long", longApolloMsg, apollo.Encoder{}},
		{"Msgpack base", baseMsgpack, encoders.Msgpack{}},
		{"Msgpack long", longMsgpack, encoders.Msgpack{}},
		{"Protobuf base", baseProtobuf, encoders.Protobuf{}},
		{"Protobuf long", longProtobuf, encoders.Protobuf{}},
	}

	for _, config := range configs {
		b.Run(config.title, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := config.encoder.Decode(config.input)
				assert.NoError(b, err, "Input: %v", config.input)
			}
		})
	}
}

func BenchmarkEncodersEncode(b *testing.B) {
	baseReply := common.Reply{Type: "message", Identifier: identifier, Message: map[string]int{"hello": 42, "world": 26}}

	payload := message{Command: "message", Identifier: longIdentifier, Message: baseReply}
	longReply := common.Reply{Type: "message", Identifier: longIdentifier, Message: payload}

	configs := []struct {
		title   string
		input   *common.Reply
		encoder encoders.Encoder
	}{
		{"JSON base", &baseReply, encoders.JSON{}},
		{"JSON long", &longReply, encoders.JSON{}},
		{"Apollo base", &baseReply, apollo.Encoder{}},
		{"Apollo long", &longReply, apollo.Encoder{}},
		{"Msgpack base", &baseReply, encoders.Msgpack{}},
		{"Msgpack long", &longReply, encoders.Msgpack{}},
		{"Protobuf base", &baseReply, encoders.Protobuf{}},
		{"Protobuf long", &longReply, encoders.Protobuf{}},
	}

	for _, config := range configs {
		b.Run(config.title, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := config.encoder.Encode(config.input)
				assert.NoError(b, err, "Input: %v", config.input)
			}
		})
	}
}

func BenchmarkEncodersEncodeTransmission(b *testing.B) {
	baseReply := message{Type: "test", Identifier: identifier, Message: map[string]int{"hello": 42, "world": 26}}
	baseJSONReplyBytes, _ := json.Marshal(&baseReply)
	baseJSONReply := string(baseJSONReplyBytes)
	longReply := message{Type: "test", Identifier: longIdentifier, Data: baseJSONReply, Message: baseReply}
	longJSONReplyBytes, _ := json.Marshal(&longReply)
	longJSONReply := string(longJSONReplyBytes)

	configs := []struct {
		title   string
		input   string
		encoder encoders.Encoder
	}{
		{"JSON base", baseJSONReply, encoders.JSON{}},
		{"JSON long", longJSONReply, encoders.JSON{}},
		{"Apollo base", baseJSONReply, apollo.Encoder{}},
		{"Apollo long", longJSONReply, apollo.Encoder{}},
		{"Msgpack base", baseJSONReply, encoders.Msgpack{}},
		{"Msgpack long", longJSONReply, encoders.Msgpack{}},
		{"Protobuf base", baseJSONReply, encoders.Protobuf{}},
		{"Protobuf long", longJSONReply, encoders.Protobuf{}},
	}

	for _, config := range configs {
		b.Run(config.title, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := config.encoder.EncodeTransmission(config.input)
				assert.NoError(b, err, "Input: %v", config.input)
			}
		})
	}
}
