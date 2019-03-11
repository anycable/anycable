package server

import (
	"net/http"

	"github.com/anycable/anycable-go/node"
	"github.com/apex/log"
	"github.com/gorilla/websocket"
)

// WebsocketHandler called when new client connection comes to websocket endpoint.
func (s *HTTPServer) WebsocketHandler(w http.ResponseWriter, r *http.Request) {
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

	// Separate goroutine for better GC of caller's data.
	go func() {
		session, err := node.NewSession(s.node, ws, r)

		if err != nil {
			ctx.Debugf("Websocket session initialization failed: %v", err)
			return
		}

		session.Log.Debug("websocket session established")

		session.ReadMessages()

		session.Log.Debug("websocket session completed")
	}()
}
