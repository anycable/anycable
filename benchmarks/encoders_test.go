package encoders

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/anycable/anycable-go/apollo"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/stretchr/testify/assert"
	"github.com/vmihailenco/msgpack/v5"
)

var (
	identifier     = "{\"channel\":\"test_channel\",\"channelId\":\"23\"}"
	longIdentifier = fmt.Sprintf("{\"channel\":\"%s\",\"channelId\":\"%s\"}", strings.Repeat("test_channel", 10), strings.Repeat("123", 10))
)

func BenchmarkEncodersDecode(b *testing.B) {
	baseCmd := common.Message{Command: "test", Identifier: identifier, Data: "hello world"}
	baseMsg, _ := json.Marshal(&baseCmd)

	longCmd := common.Message{Command: "test", Identifier: longIdentifier, Data: string(baseMsg)}
	longMsg, _ := json.Marshal(&longCmd)

	baseApolloMsg := []byte("{\"type\":\"start\",\"id\":\"abc2021\",\"payload\":{\"query\":\"Post { id }\"}}")
	longApolloMsg := []byte(fmt.Sprintf("{\"type\":\"start\",\"id\":\"%s\",\"payload\":{\"query\":%s}}", strings.Repeat("abcd_efjhkl", 10), baseApolloMsg))

	baseMsgpack, _ := msgpack.Marshal(&baseCmd)
	longMsgpack, _ := msgpack.Marshal(&longCmd)

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
	baseReply := common.Reply{Type: "test", Identifier: identifier, Message: "hello world"}
	baseJSONReply, _ := json.Marshal(&baseReply)
	longReply := common.Reply{Type: "test", Identifier: longIdentifier, Message: string(baseJSONReply)}

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
	baseReply := common.Reply{Type: "test", Identifier: identifier, Message: "hello world"}
	baseJSONReplyBytes, _ := json.Marshal(&baseReply)
	baseJSONReply := string(baseJSONReplyBytes)
	longReply := common.Reply{Type: "test", Identifier: longIdentifier, Message: baseJSONReply}
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
