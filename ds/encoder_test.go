package ds

import (
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncoder_Encode(t *testing.T) {
	t.Run("encodes data with control event", func(t *testing.T) {
		encoder := &Encoder{Cursor: "test-cursor"}

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
		assert.Contains(t, payload, `"streamNextOffset":"11::epoch1"`)
		assert.Contains(t, payload, `"streamCursor":"test-cursor"`)
	})

	t.Run("skips non-data messages", func(t *testing.T) {
		encoder := &Encoder{}

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

func TestEncoder_EncodeTransmission(t *testing.T) {
	encoder := &Encoder{}

	frame, err := encoder.EncodeTransmission(`{"message":"test"}`)
	assert.Nil(t, frame)
	assert.Nil(t, err)
}

func TestEncodeOffset(t *testing.T) {
	tests := []struct {
		name   string
		offset uint64
		epoch  string
		want   string
	}{
		{"without epoch", 123, "", "123"},
		{"with epoch", 123, "epoch1", "123::epoch1"},
		{"zero offset", 0, "epoch1", "0::epoch1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodeOffset(tt.offset, tt.epoch)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDecodeOffset(t *testing.T) {
	tests := []struct {
		name       string
		offsetStr  string
		wantOffset uint64
		wantEpoch  string
		wantErr    bool
	}{
		{"simple offset", "123", 123, "", false},
		{"with epoch", "123::epoch1", 123, "epoch1", false},
		{"start", "-1", 0, "", false},
		{"now", "now", 0, "", false},
		{"empty", "", 0, "", false},
		{"invalid", "abc", 0, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset, epoch, err := DecodeOffset(tt.offsetStr)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantOffset, offset)
				assert.Equal(t, tt.wantEpoch, epoch)
			}
		})
	}
}
