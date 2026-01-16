package ds

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/ws"
)

const dsPollEncoderID = "dspoll"

// PollEncoder is used with long polling requests.
// It's sole purpose is to properly encode a real-time message, so
// we can retain the offset information.
// It prepends the offset to the data (separated by the line break),
// so later in the connection we can restore it and set the correct headers.
type PollEncoder struct {
}

func (PollEncoder) ID() string {
	return dsPollEncoderID
}

func (PollEncoder) Encode(msg encoders.EncodedMessage) (*ws.SentFrame, error) {
	reply, isReply := msg.(*common.Reply)

	if !isReply || reply.Type != "" {
		// Skip non-data messages
		return nil, nil
	}

	// Ignore messages without offset information (they should not appear)
	if reply.Offset == 0 || reply.Epoch == "" {
		return nil, nil
	}

	dataJSON, err := json.Marshal(reply.Message)
	if err != nil {
		return nil, err
	}

	dsOffset := EncodeOffset(reply.Offset, reply.Epoch)
	payload := dsOffset + "\n" + string(dataJSON)

	return &ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte(payload)}, nil
}

func (PollEncoder) EncodeTransmission(raw string) (*ws.SentFrame, error) {
	return nil, nil
}

func (PollEncoder) Decode(raw []byte) (*common.Message, error) {
	return nil, errors.New("unsupported")
}

// PollConnection converts encoded messages into a combination of a body
// and headers on write
type PollConnection struct {
	writer http.ResponseWriter

	ctx      context.Context
	cancelFn context.CancelFunc

	done bool

	mu sync.Mutex
}

var _ node.Connection = (*PollConnection)(nil)

// NewPollConnection creates a new Durable Streams poll connection wrapper
func NewPollConnection(w http.ResponseWriter) *PollConnection {
	ctx, cancel := context.WithCancel(context.Background())
	return &PollConnection{
		writer:   w,
		ctx:      ctx,
		cancelFn: cancel,
	}
}

func (c *PollConnection) Write(msg []byte, deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.done {
		return nil
	}

	c.done = true
	defer c.cancelFn()

	// Parse the encoded message format: offset + "\n" + body
	// (see PollEncoder.Encode)
	parts := bytes.SplitN(msg, []byte("\n"), 2)
	if len(parts) != 2 {
		return errors.New("invalid poll message format: missing newline separator")
	}

	offset := string(parts[0])
	var body bytes.Buffer
	body.WriteString("[")
	body.Write(parts[1])
	body.WriteString("]")

	c.writer.Header().Set(StreamOffsetHeader, offset)
	c.writer.Header().Set(StreamUpToDateHeader, "true")

	_, err := c.writer.Write(body.Bytes())

	if err != nil {
		return err
	}

	c.writer.(http.Flusher).Flush()

	return nil
}

func (c *PollConnection) Close(code int, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.done {
		return
	}

	c.done = true
	defer c.cancelFn()

	c.writer.WriteHeader(wsCodeToHTTP(code, reason))
}

func (c *PollConnection) Context() context.Context {
	return c.ctx
}

func (c *PollConnection) Read() ([]byte, error) {
	return nil, errors.New("unsupported")
}

func (c *PollConnection) WriteBinary(msg []byte, deadline time.Time) error {
	return errors.New("unsupported")
}

func (c *PollConnection) WritePing(deadline time.Time) error {
	return nil
}

func (c *PollConnection) Descriptor() net.Conn {
	return nil
}

func wsCodeToHTTP(code int, reason string) int {
	// Only convert known WS codes
	switch code {
	case ws.CloseNormalClosure:
		if reason == "Auth Failed" {
			return http.StatusUnauthorized
		}
		return http.StatusOK
	case ws.CloseGoingAway:
		return http.StatusGone
	case ws.CloseAbnormalClosure:
		return http.StatusBadRequest
	case ws.CloseInternalServerErr:
		return http.StatusInternalServerError
	}

	return code
}
