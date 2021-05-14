package encoders

import (
	"bytes"
	"encoding/json"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/ws"
)

const msgpackEncoderID = "msgpack"

type Msgpack struct {
}

func (Msgpack) ID() string {
	return msgpackEncoderID
}

func (Msgpack) Encode(msg EncodedMessage) (*ws.SentFrame, error) {
	var b bytes.Buffer
	enc := msgpack.NewEncoder(&b)
	enc.SetCustomStructTag("json")

	err := enc.Encode(&msg)
	if err != nil {
		panic("Failed to build msgpack 😲")
	}
	return &ws.SentFrame{FrameType: ws.BinaryFrame, Payload: b.Bytes()}, nil
}

func (m Msgpack) EncodeTransmission(raw string) (*ws.SentFrame, error) {
	msg := common.Reply{}

	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		return nil, err
	}

	return m.Encode(&msg)
}

func (Msgpack) Decode(raw []byte) (*common.Message, error) {
	dec := msgpack.NewDecoder(bytes.NewReader(raw))
	dec.SetCustomStructTag("json")

	msg := &common.Message{}

	if err := dec.Decode(msg); err != nil {
		return nil, err
	}

	return msg, nil
}
