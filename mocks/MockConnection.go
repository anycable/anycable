package mocks

import (
	"errors"
	"net"
	"time"
)

type MockConnection struct {
	send   chan []byte
	closed bool
}

func (conn MockConnection) Write(msg []byte, deadline time.Time) error {
	conn.send <- msg
	return nil
}

func (conn MockConnection) WriteBinary(msg []byte, deadline time.Time) error {
	conn.send <- msg
	return nil
}

func (conn MockConnection) WritePing(deadline time.Time) error {
	return nil
}

func (conn MockConnection) Read() ([]byte, error) {
	timer := time.After(100 * time.Millisecond)

	select {
	case <-timer:
		return nil, errors.New("connection hasn't received any messages")
	case msg := <-conn.send:
		return msg, nil
	}
}

func (conn MockConnection) ReadIndifinitely() []byte {
	msg := <-conn.send
	return msg
}

func (conn MockConnection) Close(_code int, _reason string) {
	conn.send <- []byte("")
}

func (conn MockConnection) Descriptor() net.Conn {
	return nil
}

func NewMockConnection() MockConnection {
	return MockConnection{closed: false, send: make(chan []byte, 10)}
}
