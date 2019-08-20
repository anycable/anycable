package server

import (
	"net/http"
	"time"

	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/utils"
	"github.com/apex/log"
	"github.com/gorilla/websocket"
)

// WebsocketHandler generate a new http handler for WebSocket connections
func WebsocketHandler(app *node.Node, fetchHeaders []string, maxMessageSize int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := log.WithField("context", "ws")

		// TODO: make buffer sizes and compression configurable
		upgrader := websocket.Upgrader{
			// TODO: make origin check configurable
			CheckOrigin:     func(r *http.Request) bool { return true },
			Subprotocols:    []string{"actioncable-v1-json"},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}

		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			ctx.Debugf("Websocket connection upgrade error: %#v", err.Error())
			return
		}

		path := r.URL.String()
		headers := utils.FetchHeaders(r, fetchHeaders)

		uid, err := utils.FetchUID(r)
		if err != nil {
			deadline := time.Now().Add(time.Second)
			msg := websocket.FormatCloseMessage(websocket.CloseAbnormalClosure, "UID Retrieval Error")
			ws.WriteControl(websocket.CloseMessage, msg, deadline)
			ws.Close()
			return
		}

		ws.SetReadLimit(maxMessageSize)

		// Separate goroutine for better GC of caller's data.
		go func() {
			session, err := node.NewSession(app, ws, path, headers, uid)

			if err != nil {
				ctx.Debugf("Websocket session initialization failed: %v", err)
				return
			}

			session.Log.Debug("websocket session established")

			session.ReadMessages()

			session.Log.Debug("websocket session completed")
		}()
	})
}
