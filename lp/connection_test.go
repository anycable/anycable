package lp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anycable/anycable-go/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (c *Connection) testResponse() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.writer.(*httptest.ResponseRecorder).Body.String()
}

func (c *Connection) testStatus() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.writer.(*httptest.ResponseRecorder).Code
}

func TestConnection_Write(t *testing.T) {
	w := httptest.NewRecorder()
	c := NewConnection(500)
	c.ResetWriter(w)

	msg := []byte("hello, world!")
	err := c.Write(msg, time.Now().Add(1*time.Second))

	assert.NoError(t, err)

	c.Flush()

	// Wait for a bit
	time.Sleep(60 * time.Millisecond)

	// Make sure we do not write immediately
	assert.Equal(t, "", c.testResponse())

	<-c.Context().Done()

	assert.Equal(t, http.StatusOK, c.testStatus())
	assert.Equal(t, "hello, world!\n", c.testResponse())
}

func TestConnection_Close(t *testing.T) {
	t.Run("Normal closure", func(t *testing.T) {
		w := httptest.NewRecorder()
		c := NewConnection(100)
		c.ResetWriter(w)

		c.Close(ws.CloseNormalClosure, "bye")

		assert.Equal(t, http.StatusOK, c.testStatus())
		assert.Equal(t, "", c.testResponse())
	})

	t.Run("Abnormal closure", func(t *testing.T) {
		// Create a new connection
		w := httptest.NewRecorder()
		c := NewConnection(100)
		c.ResetWriter(w)

		c.Close(ws.CloseAbnormalClosure, "bye")

		assert.Equal(t, http.StatusInternalServerError, c.testStatus())
		assert.Equal(t, "", c.testResponse())
	})

	t.Run("Close with HTTP status", func(t *testing.T) {
		// Create a new connection
		w := httptest.NewRecorder()
		c := NewConnection(100)
		c.ResetWriter(w)

		c.Close(http.StatusNoContent, "No content")

		assert.Equal(t, http.StatusNoContent, c.testStatus())
		assert.Equal(t, "", c.testResponse())
	})

	t.Run("Authentication failure closure", func(t *testing.T) {
		// Create a new connection
		w := httptest.NewRecorder()
		c := NewConnection(100)
		c.ResetWriter(w)

		err := c.Write([]byte("unauthorized"), time.Now().Add(1*time.Second))
		assert.NoError(t, err)

		c.Close(ws.CloseNormalClosure, "Auth Failed")

		assert.Equal(t, http.StatusUnauthorized, c.testStatus())
		assert.Equal(t, "unauthorized\n", c.testResponse())
	})
}

func TestConnection_WriteBinary(t *testing.T) {
	w := httptest.NewRecorder()
	c := NewConnection(0)
	c.ResetWriter(w)

	msg := []byte{0x01, 0x02, 0x03}
	err := c.WriteBinary(msg, time.Now().Add(1*time.Second))

	// Wait for a bit
	time.Sleep(10 * time.Millisecond)

	assert.Error(t, err)
	assert.Equal(t, []byte(nil), w.Body.Bytes())
}

func TestConnection_Read(t *testing.T) {
	c := NewConnection(0)

	msg, err := c.Read()

	assert.Error(t, err)
	assert.Equal(t, []byte(nil), msg)
}

func TestWsCodeToHTTP(t *testing.T) {
	// Verify that WebSocket codes are mapped to HTTP codes correctly
	assert.Equal(t, http.StatusOK, wsCodeToHTTP(ws.CloseNormalClosure, "Ok"))
	assert.Equal(t, http.StatusUnauthorized, wsCodeToHTTP(ws.CloseNormalClosure, "Auth Failed"))
	assert.Equal(t, http.StatusServiceUnavailable, wsCodeToHTTP(ws.CloseGoingAway, "Server Restart"))
	assert.Equal(t, http.StatusInternalServerError, wsCodeToHTTP(ws.CloseAbnormalClosure, "Internal Error"))
	assert.Equal(t, http.StatusInternalServerError, wsCodeToHTTP(ws.CloseInternalServerErr, "Internal Error"))
}

func TestConnection_Descriptor(t *testing.T) {
	c := NewConnection(0)

	assert.Nil(t, c.Descriptor())
}

func TestConnection_ResetWriter(t *testing.T) {
	w := httptest.NewRecorder()
	c := NewConnection(50)
	c.ResetWriter(w)

	msg := []byte("hello, world!")
	err := c.Write(msg, time.Now().Add(1*time.Second))

	require.NoError(t, err)

	c.Flush()
	<-c.Context().Done()

	assert.Equal(t, http.StatusOK, c.testStatus())
	assert.Equal(t, "hello, world!\n", c.testResponse())

	// Now write again
	err = c.Write([]byte("bye-bye"), time.Now().Add(1*time.Second))
	require.NoError(t, err)

	// Reset writer again -> that should trigger flush, since buffer is non-empty
	w2 := httptest.NewRecorder()
	c.ResetWriter(w2)

	<-c.Context().Done()

	assert.Equal(t, http.StatusOK, c.testStatus())
	assert.Equal(t, "bye-bye\n", c.testResponse())
}
