package pool

import (
	"errors"
	"fmt"
	"sync"

	"google.golang.org/grpc"
)

var (
	errClosed = errors.New("pool is closed")
)

// Pool represents connection pool
type Pool interface {
	Get() (Conn, error)
	Close()
	Available() int
	Busy() int
}

type channelPool struct {
	mu          sync.Mutex
	conns       chan *grpc.ClientConn
	activeCount int
	factory     Factory
}

// Factory is a connection pool factory fun interface
type Factory func() (*grpc.ClientConn, error)

// Conn is a single connection to pool wrapper
type Conn struct {
	Conn *grpc.ClientConn
	c    *channelPool
}

// Close connection
func (p Conn) Close() error {
	return p.c.put(p.Conn)
}

func (c *channelPool) wrapConn(conn *grpc.ClientConn) Conn {
	c.mu.Lock()
	p := Conn{Conn: conn, c: c}
	c.activeCount++
	c.mu.Unlock()
	return p
}

// NewChannelPool builds a new pool with provided configuration
func NewChannelPool(initialCap, maxCap int, factory Factory) (Pool, error) {
	if initialCap < 0 || maxCap <= 0 || initialCap > maxCap {
		return nil, errors.New("invalid capacity settings")
	}

	c := &channelPool{
		conns:       make(chan *grpc.ClientConn, maxCap),
		factory:     factory,
		activeCount: 0,
	}

	for i := 0; i < initialCap; i++ {
		conn, err := factory()
		if err != nil {
			c.Close()
			return nil, fmt.Errorf("factory is not able to fill the pool: %s", err)
		}
		c.conns <- conn
	}

	return c, nil
}

func (c *channelPool) getConns() chan *grpc.ClientConn {
	c.mu.Lock()
	conns := c.conns
	c.mu.Unlock()
	return conns
}

func (c *channelPool) Get() (Conn, error) {
	conns := c.getConns()
	if conns == nil {
		return Conn{}, errClosed
	}

	// wrap our connections with out custom grpc.ClientConn implementation (wrapConn
	// method) that puts the connection back to the pool if it's closed.
	select {
	case conn := <-conns:
		if conn == nil {
			return Conn{}, errClosed
		}

		return c.wrapConn(conn), nil
	default:
		conn, err := c.factory()
		if err != nil {
			return Conn{}, err
		}

		return c.wrapConn(conn), nil
	}
}

func (c *channelPool) put(conn *grpc.ClientConn) error {
	if conn == nil {
		return errors.New("connection is nil. rejecting")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.activeCount--

	if c.conns == nil {
		// pool is closed, close passed connection
		return conn.Close()
	}

	// put the resource back into the pool. If the pool is full, this will
	// block and the default case will be executed.
	select {
	case c.conns <- conn:
		return nil
	default:
		// pool is full, close passed connection
		return conn.Close()
	}
}

// Close all connections and pool's channel
func (c *channelPool) Close() {
	c.mu.Lock()
	conns := c.conns
	c.conns = nil
	c.factory = nil
	c.mu.Unlock()

	if conns == nil {
		return
	}

	close(conns)
	for conn := range conns {
		conn.Close()
	}
}

func (c *channelPool) Busy() int { return c.activeCount }

func (c *channelPool) Available() int { return len(c.getConns()) }
