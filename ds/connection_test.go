package ds

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnection_Write(t *testing.T) {
	t.Run("writes with SSE separators", func(t *testing.T) {
		w := httptest.NewRecorder()
		conn := NewConnection(w)

		err := conn.Write([]byte("test"), time.Time{})
		require.NoError(t, err)

		// Message should be in backlog before established
		assert.Equal(t, "test\n\n", conn.backlog.String())
		assert.Equal(t, 0, w.Body.Len())

		conn.Established()

		// After established, backlog should be flushed
		assert.Equal(t, "test\n\n", w.Body.String())
		assert.Equal(t, 0, conn.backlog.Len())
	})

	t.Run("accumulates messages before established", func(t *testing.T) {
		w := httptest.NewRecorder()
		conn := NewConnection(w)

		conn.Write([]byte("msg1"), time.Time{}) // nolint: errcheck
		conn.Write([]byte("msg2"), time.Time{}) // nolint: errcheck

		assert.Equal(t, "msg1\n\nmsg2\n\n", conn.backlog.String())

		conn.Established()

		assert.Equal(t, "msg1\n\nmsg2\n\n", w.Body.String())
	})
}

func TestConnection_Close(t *testing.T) {
	w := httptest.NewRecorder()
	conn := NewConnection(w)

	conn.Close(0, "test")

	assert.True(t, conn.done)

	// Writing after close should not error
	err := conn.Write([]byte("test"), time.Time{})
	assert.NoError(t, err)

	// But should not write anything
	assert.Equal(t, 0, w.Body.Len())
}
