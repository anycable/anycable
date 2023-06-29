package lp

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/ws"
)

type Connection struct {
	writer   http.ResponseWriter
	ctx      context.Context
	cancelFn context.CancelFunc
	// Defines if the connection is no longer processing messages (closed or flushed)
	done bool
	// Defines if the connection is ready to flush messages
	ready bool

	backlog       *bytes.Buffer
	flushInterval time.Duration
	flushTimer    *time.Timer

	mu sync.Mutex
}

var _ node.Connection = (*Connection)(nil)

// NewConnection creates a new long-polling connection wrapper
func NewConnection(flushMs int) *Connection {
	flushInterval := time.Duration(flushMs) * time.Millisecond

	return &Connection{
		backlog:       bytes.NewBuffer(nil),
		flushInterval: flushInterval,
	}
}

func (c *Connection) Read() ([]byte, error) {
	return nil, errors.New("unsupported")
}

func (c *Connection) Write(msg []byte, deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.backlog.Write(msg)
	c.backlog.Write([]byte("\n"))

	c.scheduleFlush()

	return nil
}

func (c *Connection) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ready = true

	if c.backlog.Len() > 0 {
		c.scheduleFlush()
	}
}

func (c *Connection) scheduleFlush() {
	if c.done || !c.ready {
		return
	}

	if c.flushTimer == nil {
		c.flushTimer = time.AfterFunc(c.flushInterval, c.flush)
	}
}

func (c *Connection) flush() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.done {
		return
	}

	c.done = true
	c.ready = false
	c.flushTimer = nil

	_, err := c.writer.Write(c.backlog.Bytes())

	if err == nil {
		c.backlog.Reset()
	}

	c.cancelFn()
}

func (c *Connection) WriteBinary(msg []byte, deadline time.Time) error {
	return errors.New("unsupported")
}

func (c *Connection) Close(code int, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.done {
		return
	}

	if c.flushTimer != nil {
		c.flushTimer.Stop()
		c.flushTimer = nil
		c.ready = false
	}

	c.done = true

	c.writer.WriteHeader(wsCodeToHTTP(code, reason))

	// Flush messages (e.g., {"type":"disconnect"})
	if c.backlog.Len() > 0 {
		_, err := c.writer.Write(c.backlog.Bytes())

		if err == nil {
			c.backlog.Reset()
		}
	}

	c.cancelFn()
}

// Mark as closed to avoid writing to closed connection
func (c *Connection) Closed() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.done = true
}

func (c *Connection) Descriptor() net.Conn {
	return nil
}

func (c *Connection) ResetWriter(w http.ResponseWriter) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.writer = w
	c.done = false

	ctx, cancel := context.WithCancel(context.Background())
	c.cancelFn = cancel
	c.ctx = ctx

	if c.backlog.Len() > 0 {
		if c.flushTimer == nil {
			c.flushTimer = time.AfterFunc(c.flushInterval, c.flush)
		}
	}
}

func (c *Connection) Context() context.Context {
	return c.ctx
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
		return http.StatusServiceUnavailable
	case ws.CloseAbnormalClosure:
		return http.StatusInternalServerError
	case ws.CloseInternalServerErr:
		return http.StatusInternalServerError
	}

	return code
}
