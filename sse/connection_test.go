package sse

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anycable/anycable-go/ws"
	"github.com/stretchr/testify/assert"
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
	c := NewConnection(w)

	msg := []byte("hello, world!")
	err := c.Write(msg, time.Now().Add(1*time.Second))

	assert.NoError(t, err)

	assert.Empty(t, c.testResponse())

	c.Established()

	assert.Equal(t, http.StatusOK, c.testStatus())
	assert.Equal(t, "hello, world!\n\n", c.testResponse())
}

func TestConnection_Close(t *testing.T) {
	t.Run("Close cancels the context", func(t *testing.T) {
		// Create a new connection
		w := httptest.NewRecorder()
		c := NewConnection(w)

		ctx := c.Context()

		c.Close(ws.CloseNormalClosure, "bye")

		<-ctx.Done()
		assert.True(t, c.done)

		c.Close(ws.CloseNormalClosure, "bye")
	})
}

func TestConnection_WriteBinary(t *testing.T) {
	w := httptest.NewRecorder()
	c := NewConnection(w)

	msg := []byte{0x01, 0x02, 0x03}
	err := c.WriteBinary(msg, time.Now().Add(1*time.Second))

	assert.Error(t, err)
	assert.Equal(t, []byte(nil), w.Body.Bytes())
}

func TestConnection_Read(t *testing.T) {
	w := httptest.NewRecorder()
	c := NewConnection(w)

	msg, err := c.Read()

	assert.Error(t, err)
	assert.Equal(t, []byte(nil), msg)
}

func TestConnection_Descriptor(t *testing.T) {
	w := httptest.NewRecorder()
	c := NewConnection(w)

	assert.Nil(t, c.Descriptor())
}
