package node

import (
	"testing"

	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/ws"
	"github.com/stretchr/testify/assert"
)

type MockMessage struct {
	Encoded int
	Value   string
}

func (m *MockMessage) GetType() string {
	return "mock"
}

func TestEncodingCache(t *testing.T) {
	msg := &MockMessage{Value: "mock"}

	c := NewEncodingCache()

	callback := func(msg encoders.EncodedMessage) (*ws.SentFrame, error) {
		if m, ok := msg.(*MockMessage); ok {
			m.Encoded++
			return &ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte(m.Value)}, nil
		}
		return nil, nil
	}

	v, _ := c.Fetch(msg, "mock", callback)
	assert.Equal(t, []byte("mock"), v.Payload)

	v, _ = c.Fetch(msg, "mock", callback)
	assert.Equal(t, []byte("mock"), v.Payload)
	assert.Equal(t, 1, msg.Encoded)
}
