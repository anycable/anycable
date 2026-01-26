package ds

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/ws"
)

const dsSSEEncoderID = "dsse"

type SSEEncoder struct {
	Cursor   string
	UpToDate bool
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

	// Encode data event - wrap in array to match catchup format
	dataJSON, err := json.Marshal(reply.Message)
	if err != nil {
		return nil, err
	}

	payload := "event: data\ndata:[" + string(dataJSON) + "]"

	control := make(map[string]interface{})
	control["streamNextOffset"] = EncodeOffset(reply.Offset, reply.Epoch)

	if e.Cursor != "" {
		control["streamCursor"] = e.Cursor
	}

	if e.UpToDate {
		control["upToDate"] = true
	}

	controlJSON, _ := json.Marshal(control)
	payload += "\n\nevent: control\ndata:" + string(controlJSON)

	return &ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte(payload)}, nil
}

func (e *SSEEncoder) EncodeTransmission(raw string) (*ws.SentFrame, error) {
	return nil, nil
}

func (e *SSEEncoder) Decode(raw []byte) (*common.Message, error) {
	return nil, errors.New("unsupported")
}

// EncodeSSEDataEvent encodes a batch of messages as an SSE data event
func EncodeSSEDataEvent(messages []common.StreamMessage) []byte {
	var buf bytes.Buffer
	buf.WriteString("event: data\ndata:")
	buf.Write(EncodeJSONBatch(messages))
	return buf.Bytes()
}

// EncodeSSEControlEvent encodes a control event with offset, cursor, and upToDate flag
func EncodeSSEControlEvent(offset, cursor string, upToDate bool) []byte {
	control := make(map[string]interface{})
	control["streamNextOffset"] = offset

	if cursor != "" {
		control["streamCursor"] = cursor
	}

	if upToDate {
		control["upToDate"] = true
	}

	controlJSON, _ := json.Marshal(control)

	var buf bytes.Buffer
	buf.WriteString("event: control\ndata:")
	buf.Write(controlJSON)
	return buf.Bytes()
}
