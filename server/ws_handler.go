package server

import (
	"net/http"

	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/utils"
	"github.com/apex/log"
	"github.com/gorilla/websocket"
)

// WSConfig contains WebSocket connection configuration.
type WSConfig struct {
	ReadBufferSize    int
	WriteBufferSize   int
	MaxMessageSize    int64
	EnableCompression bool
}

// NewWSConfig build a new WSConfig struct
func NewWSConfig() WSConfig {
	return WSConfig{}
}

// WebsocketHandler generate a new http handler for WebSocket connections
func WebsocketHandler(app *node.Node, fetchHeaders []string, config *WSConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := log.WithField("context", "ws")

		upgrader := websocket.Upgrader{
			CheckOrigin:       func(r *http.Request) bool { return true },
			Subprotocols:      []string{"actioncable-v1-json"},
			ReadBufferSize:    config.ReadBufferSize,
			WriteBufferSize:   config.WriteBufferSize,
			EnableCompression: config.EnableCompression,
		}

		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			ctx.Debugf("Websocket connection upgrade error: %#v", err.Error())
			return
		}

		url := r.URL.String()
		headers := utils.FetchHeaders(r, fetchHeaders)

		uid, err := utils.FetchUID(r)
		if err != nil {
			utils.CloseWS(ws, websocket.CloseAbnormalClosure, "UID Retrieval Error")
			return
		}

		ws.SetReadLimit(config.MaxMessageSize)

		if config.EnableCompression {
			ws.EnableWriteCompression(true)
		}

		// Separate goroutine for better GC of caller's data.
		go func() {
			session, err := node.NewSession(app, ws, url, headers, uid)

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
