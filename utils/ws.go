package utils

import (
	"time"

	"github.com/gorilla/websocket"
)

// CloseWS closes WebSocket connection with the specified close code and reason
func CloseWS(ws *websocket.Conn, code int, reason string) {
	deadline := time.Now().Add(time.Second)
	msg := websocket.FormatCloseMessage(code, reason)
	ws.WriteControl(websocket.CloseMessage, msg, deadline) //nolint:errcheck
	ws.Close()
}
