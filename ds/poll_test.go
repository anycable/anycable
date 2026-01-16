package ds

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPollEncoder_Encode(t *testing.T) {
	encoder := PollEncoder{}

	t.Run("returns nil for non-reply messages", func(t *testing.T) {
		msg := &common.PingMessage{Type: "ping", Message: 123}
		frame, err := encoder.Encode(msg)

		assert.NoError(t, err)
		assert.Nil(t, frame)
	})

	t.Run("returns nil for reply with type", func(t *testing.T) {
		reply := &common.Reply{Type: "confirm_subscription", Offset: 1, Epoch: "epoch1"}
		frame, err := encoder.Encode(reply)

		assert.NoError(t, err)
		assert.Nil(t, frame)
	})

	t.Run("returns nil for reply without offset", func(t *testing.T) {
		reply := &common.Reply{Message: "test"}
		frame, err := encoder.Encode(reply)

		assert.NoError(t, err)
		assert.Nil(t, frame)
	})

	t.Run("encodes reply with offset and epoch", func(t *testing.T) {
		reply := &common.Reply{
			Message: map[string]interface{}{"data": "test"},
			Offset:  123,
			Epoch:   "epoch1",
		}
		frame, err := encoder.Encode(reply)

		require.NoError(t, err)
		require.NotNil(t, frame)
		assert.Equal(t, "123::epoch1\n{\"data\":\"test\"}", string(frame.Payload))
	})
}

func TestPollEncoder_ID(t *testing.T) {
	encoder := PollEncoder{}
	assert.Equal(t, "dspoll", encoder.ID())
}

func TestPollConnection_Write(t *testing.T) {
	t.Run("parses message and sets offset header", func(t *testing.T) {
		w := httptest.NewRecorder()
		conn := NewPollConnection(w)

		err := conn.Write([]byte("123::epoch1\n{\"data\":\"test\"}"), time.Time{})
		require.NoError(t, err)

		assert.Equal(t, "123::epoch1", w.Header().Get(StreamOffsetHeader))
		assert.Equal(t, "[{\"data\":\"test\"}]", w.Body.String())
	})

	t.Run("handles body with newlines", func(t *testing.T) {
		w := httptest.NewRecorder()
		conn := NewPollConnection(w)

		err := conn.Write([]byte("42::epoch2\n{\"text\":\"line1\\nline2\"}"), time.Time{})
		require.NoError(t, err)

		assert.Equal(t, "42::epoch2", w.Header().Get(StreamOffsetHeader))
		assert.Equal(t, "[{\"text\":\"line1\\nline2\"}]", w.Body.String())
	})

	t.Run("returns error for invalid format", func(t *testing.T) {
		w := httptest.NewRecorder()
		conn := NewPollConnection(w)

		err := conn.Write([]byte("no-newline-here"), time.Time{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid poll message format")
	})
}

func TestPollConnection_Close(t *testing.T) {
	w := httptest.NewRecorder()
	conn := NewPollConnection(w)

	conn.Close(ws.CloseNormalClosure, "test")

	assert.True(t, conn.done)

	// Writing after close should not error
	err := conn.Write([]byte("123::epoch1\ntest"), time.Time{})
	assert.NoError(t, err)

	// But should not write anything
	assert.Equal(t, 0, w.Body.Len())
}
