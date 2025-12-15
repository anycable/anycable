package ds

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/ws"
)

const (
	dsEncoderID     = "ds"
	offsetSeparator = "::"

	StartOffset = "-1"
)

// EncodeOffset encodes offset and epoch into a single opaque offset string
// Format: <offset>::<epoch> to maintain lexicographic ordering
func EncodeOffset(offset uint64, epoch string) string {
	if epoch == "" {
		return fmt.Sprintf("%d", offset)
	}
	return fmt.Sprintf("%d%s%s", offset, offsetSeparator, epoch)
}

// DecodeOffset decodes an opaque offset string into offset number and epoch
// Returns (0, "", nil) for start-of-stream markers: "", "0", "-1", "now"
func DecodeOffset(offsetStr string) (uint64, string, error) {
	if offsetStr == "" || offsetStr == "0" || offsetStr == StartOffset || offsetStr == "now" {
		return 0, "", nil
	}

	parts := strings.Split(offsetStr, offsetSeparator)
	offset, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, "", err
	}

	var epoch string
	if len(parts) > 1 {
		epoch = parts[1]
	}

	return offset, epoch, nil
}

// Encoder is responsible for converting messages to Durable Streams format
// It supports SSE (event stream) mode only for live streaming
type Encoder struct {
	// Cursor to echo back in control messages (for CDN collapsing)
	Cursor string
}

func (Encoder) ID() string {
	return dsEncoderID
}

func (e *Encoder) Encode(msg encoders.EncodedMessage) (*ws.SentFrame, error) {
	reply, isReply := msg.(*common.Reply)

	if !isReply || reply.Type != "" {
		// Skip non-data messages
		return nil, nil
	}

	// Encode data event
	dataJSON, err := json.Marshal(reply.Message)
	if err != nil {
		return nil, err
	}

	payload := "event: data\ndata: " + string(dataJSON)

	// Append control event with offset and cursor information
	if reply.Offset > 0 || reply.Epoch != "" || e.Cursor != "" {
		control := make(map[string]interface{})

		// Encode offset with epoch
		if reply.Offset > 0 || reply.Epoch != "" {
			nextOffset := reply.Offset
			if reply.StreamID != "" {
				// For streamed replies, offset is already set correctly
				nextOffset = reply.Offset + 1
			}
			control["streamNextOffset"] = EncodeOffset(nextOffset, reply.Epoch)
		}

		if e.Cursor != "" {
			control["streamCursor"] = e.Cursor
		}

		controlJSON, _ := json.Marshal(control)
		payload += "\n\nevent: control\ndata: " + string(controlJSON)
	}

	return &ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte(payload)}, nil
}

func (e Encoder) EncodeTransmission(raw string) (*ws.SentFrame, error) {
	// DS doesn't use Action Cable protocol messages
	return nil, nil
}

func (Encoder) Decode(raw []byte) (*common.Message, error) {
	return nil, errors.New("unsupported")
}
