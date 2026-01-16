package ds

import (
	"encoding/json"
	"errors"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/ws"
)

const dsSSEEncoderID = "dsse"

type SSEEncoder struct {
	Cursor string
}

func (e *SSEEncoder) ID() string {
	return dsSSEEncoderID
}

func (e *SSEEncoder) Encode(msg encoders.EncodedMessage) (*ws.SentFrame, error) {
	reply, isReply := msg.(*common.Reply)

	if !isReply || reply.Type != "" {
		// Skip non-data messages
		return nil, nil
	}

	// Ignore messages without offset information (they should not appear)
	if reply.Offset == 0 || reply.Epoch == "" {
		return nil, nil
	}

	// Encode data event
	dataJSON, err := json.Marshal(reply.Message)
	if err != nil {
		return nil, err
	}

	payload := "event: data\ndata: " + string(dataJSON)

	control := make(map[string]interface{})
	control["streamNextOffset"] = EncodeOffset(reply.Offset, reply.Epoch)

	if e.Cursor != "" {
		control["streamCursor"] = e.Cursor
	}

	controlJSON, _ := json.Marshal(control)
	payload += "\n\nevent: control\ndata: " + string(controlJSON)

	return &ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte(payload)}, nil
}

func (e *SSEEncoder) EncodeTransmission(raw string) (*ws.SentFrame, error) {
	return nil, nil
}

func (e *SSEEncoder) Decode(raw []byte) (*common.Message, error) {
	return nil, errors.New("unsupported")
}
