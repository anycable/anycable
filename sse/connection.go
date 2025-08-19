package sse

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/anycable/anycable-go/node"
)

type Connection struct {
	writer http.ResponseWriter

	ctx      context.Context
	cancelFn context.CancelFunc

	done        bool
	established bool
	// Backlog is used to store messages sent to client before connection is established
	backlog *bytes.Buffer

	mu sync.Mutex
}

var _ node.Connection = (*Connection)(nil)

// NewConnection creates a new long-polling connection wrapper
func NewConnection(w http.ResponseWriter) *Connection {
	ctx, cancel := context.WithCancel(context.Background())
	return &Connection{
		writer:   w,
		backlog:  bytes.NewBuffer(nil),
		ctx:      ctx,
		cancelFn: cancel,
	}
}

func (c *Connection) Read() ([]byte, error) {
	return nil, errors.New("unsupported")
}

func (c *Connection) Write(msg []byte, deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.done {
		return nil
	}

	if !c.established {
		c.backlog.Write(msg)
		c.backlog.Write([]byte("\n\n"))
		return nil
	}

	_, err := c.writer.Write(msg)

	if err != nil {
		return err
	}

	_, err = c.writer.Write([]byte("\n\n"))

	if err != nil {
		return err
	}

	c.writer.(http.Flusher).Flush()

	return nil
}

func (c *Connection) WriteBinary(msg []byte, deadline time.Time) error {
	return errors.New("unsupported")
}

func (c *Connection) WritePing(deadline time.Time) error {
	return nil
}

func (c *Connection) Context() context.Context {
	return c.ctx
}

func (c *Connection) Close(code int, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.done {
		return
	}

	c.done = true

	c.cancelFn()
}

// Mark as closed to avoid writing to closed connection
func (c *Connection) Established() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.established = true

	if c.backlog.Len() > 0 {
		c.writer.Write(c.backlog.Bytes()) // nolint: errcheck
		c.writer.(http.Flusher).Flush()
		c.backlog.Reset()
	}
}

func (c *Connection) Descriptor() net.Conn {
	return nil
}
