package ws

import (
	"net"
	"time"

	"github.com/gorilla/websocket"
)

// Connection is a WebSocket implementation of Connection
type Connection struct {
	conn *websocket.Conn
}

func NewConnection(conn *websocket.Conn) *Connection {
	return &Connection{conn}
}

// Write writes a text message to a WebSocket
func (ws Connection) Write(msg []byte, deadline time.Time) error {
	if err := ws.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}

	w, err := ws.conn.NextWriter(websocket.TextMessage)

	if err != nil {
		return err
	}

	if _, err = w.Write(msg); err != nil {
		return err
	}

	return w.Close()
}

// WriteBinary writes a binary message to a WebSocket
func (ws Connection) WriteBinary(msg []byte, deadline time.Time) error {
	if err := ws.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}

	w, err := ws.conn.NextWriter(websocket.BinaryMessage)

	if err != nil {
		return err
	}

	if _, err = w.Write(msg); err != nil {
		return err
	}

	return w.Close()
}

func (ws Connection) Read() ([]byte, error) {
	_, message, err := ws.conn.ReadMessage()
	return message, err
}

// Close sends close frame with a given code and a reason
func (ws Connection) Close(code int, reason string) {
	CloseWithReason(ws.conn, code, reason)
}

func (ws Connection) Descriptor() net.Conn {
	return ws.conn.UnderlyingConn()
}

// CloseWithReason closes WebSocket connection with the specified close code and reason
func CloseWithReason(ws *websocket.Conn, code int, reason string) {
	deadline := time.Now().Add(time.Second)
	msg := websocket.FormatCloseMessage(code, reason)
	ws.WriteControl(websocket.CloseMessage, msg, deadline) //nolint:errcheck
	ws.Close()
}
