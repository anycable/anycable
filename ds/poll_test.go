package ds

import (
	"net/http"
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

	t.Run("sets cache control header", func(t *testing.T) {
		w := httptest.NewRecorder()
		conn := NewPollConnection(w)
		conn.CacheControl = "public, max-age=60"

		err := conn.Write([]byte("123::epoch1\n{\"data\":\"test\"}"), time.Time{})
		require.NoError(t, err)

		assert.Equal(t, "public, max-age=60", w.Header().Get("Cache-Control"))
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

func TestPollConnection_CloseCacheControl(t *testing.T) {
	t.Run("disables caching for no content", func(t *testing.T) {
		w := httptest.NewRecorder()
		conn := NewPollConnection(w)
		conn.CacheControl = "public, max-age=60"

		conn.Close(http.StatusNoContent, "No content")

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	})

	t.Run("disables caching for errors", func(t *testing.T) {
		w := httptest.NewRecorder()
		conn := NewPollConnection(w)

		conn.Close(http.StatusGone, "")

		assert.Equal(t, http.StatusGone, w.Code)
		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	})

	t.Run("keeps cache control for OK", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.Header().Set("Cache-Control", "public, max-age=60")
		conn := NewPollConnection(w)

		conn.Close(http.StatusOK, "")

		assert.Equal(t, "public, max-age=60", w.Header().Get("Cache-Control"))
	})
}
