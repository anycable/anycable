package pool

import (
  "errors"
  "fmt"
  "sync"

  "google.golang.org/grpc"
)

var (
  ErrClosed = errors.New("pool is closed")
)

type Pool interface {
  Get() (PoolConn, error)
  Close()
  Len() int
}

type channelPool struct {
  mu    sync.Mutex
  conns chan *grpc.ClientConn
  factory Factory
}

type Factory func() (*grpc.ClientConn, error)

type PoolConn struct {
  Conn *grpc.ClientConn
  c *channelPool
}

func (p PoolConn) Close() error {
  return p.c.put(p.Conn)
}

func (c *channelPool) wrapConn(conn *grpc.ClientConn) PoolConn {
  p := PoolConn{Conn: conn, c: c}
  return p
}

func NewChannelPool(initialCap, maxCap int, factory Factory) (Pool, error) {
  if initialCap < 0 || maxCap <= 0 || initialCap > maxCap {
    return nil, errors.New("invalid capacity settings")
  }

  c := &channelPool{
    conns:   make(chan *grpc.ClientConn, maxCap),
    factory: factory,
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

func (c *channelPool) Get() (PoolConn, error) {
  conns := c.getConns()
  if conns == nil {
    return PoolConn{}, ErrClosed
  }

  // wrap our connections with out custom grpc.ClientConn implementation (wrapConn
  // method) that puts the connection back to the pool if it's closed.
  select {
  case conn := <-conns:
    if conn == nil {
      return PoolConn{}, ErrClosed
    }

    return c.wrapConn(conn), nil
  default:
    conn, err := c.factory()
    if err != nil {
      return PoolConn{}, err
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

func (c *channelPool) Len() int { return len(c.getConns()) }