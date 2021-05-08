package encoders

import (
	"encoding/json"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/ws"
)

const jsonEncoderID = "json"

type JSON struct {
}

func (JSON) Encode(msg EncodedMessage) (*ws.SentFrame, error) {
	return &ws.SentFrame{FrameType: ws.TextFrame, Payload: msg.ToJSON()}, nil
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
