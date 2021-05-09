package encoders

import (
	"encoding/json"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/ws"
)

const jsonEncoderID = "json"

type JSON struct {
}

func (JSON) ID() string {
	return jsonEncoderID
}

func (JSON) Encode(msg EncodedMessage) (*ws.SentFrame, error) {
	b, err := json.Marshal(&msg)
	if err != nil {
		panic("Failed to build JSON 😲")
	}
	return &ws.SentFrame{FrameType: ws.TextFrame, Payload: b}, nil
}

func (JSON) EncodeTransmission(msg string) (*ws.SentFrame, error) {
	return &ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte(msg)}, nil
}

func (JSON) Decode(raw []byte) (*common.Message, error) {
	msg := &common.Message{}

	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, err
	}

	return msg, nil
}
