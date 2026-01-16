package ds

import (
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSEEncoder_Encode(t *testing.T) {
	t.Run("encodes data with control event", func(t *testing.T) {
		encoder := &SSEEncoder{Cursor: "test-cursor"}

		msg := &common.Reply{
			Message:  map[string]string{"id": "1", "text": "hello"},
			Offset:   10,
			Epoch:    "epoch1",
			StreamID: "test",
		}

		frame, err := encoder.Encode(msg)
		require.NoError(t, err)
		require.NotNil(t, frame)

		payload := string(frame.Payload)

		assert.Contains(t, payload, "event: data")
		assert.Contains(t, payload, `"id":"1"`)
		assert.Contains(t, payload, `"text":"hello"`)

		assert.Contains(t, payload, "event: control")
		assert.Contains(t, payload, `"streamNextOffset":"10::epoch1"`)
		assert.Contains(t, payload, `"streamCursor":"test-cursor"`)
	})

	t.Run("skips non-data messages", func(t *testing.T) {
		encoder := &SSEEncoder{}

		msg := &common.Reply{
			Type: "welcome",
		}

		frame, err := encoder.Encode(msg)
		require.NoError(t, err)
		assert.Nil(t, frame)

		msg = &common.Reply{
			Type: "disconnect",
		}

		frame, err = encoder.Encode(msg)
		require.NoError(t, err)
		assert.Nil(t, frame)
	})
}

func TestSSEEncoder_EncodeTransmission(t *testing.T) {
	encoder := &SSEEncoder{}

	frame, err := encoder.EncodeTransmission(`{"message":"test"}`)
	assert.Nil(t, frame)
	assert.Nil(t, err)
}
