package ds

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/ws"
)

const (
	dsNoopEncoderID = "dsnull"

	offsetSeparator = "::"

	StartOffset = "-1"
)

// EncodeOffset encodes offset and epoch into a single opaque offset string
// Format: <offset>::<epoch> to maintain lexicographic ordering
func EncodeOffset(offset uint64, epoch string) string {
	// Empty stream
	if epoch == "" {
		return "0"
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

func EncodeJSONBatch(batch []common.StreamMessage) []byte {
	var buf bytes.Buffer
	buf.WriteString("[")
	for i, reply := range batch {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.Write([]byte(reply.Data))
	}
	buf.WriteString("]")
	return buf.Bytes()
}

// NoopEncoder is used with one-shot HTTP requests
// where all messages are controlled by us (not by AnyCable)
type NoopEncoder struct {
}

func (NoopEncoder) ID() string {
	return dsNoopEncoderID
}

func (NoopEncoder) Encode(msg encoders.EncodedMessage) (*ws.SentFrame, error) {
	return nil, nil
}

func (NoopEncoder) EncodeTransmission(raw string) (*ws.SentFrame, error) {
	return nil, nil
}

func (NoopEncoder) Decode(raw []byte) (*common.Message, error) {
	return nil, errors.New("unsupported")
}
