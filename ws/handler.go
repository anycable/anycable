package ws

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/version"
	"github.com/gorilla/websocket"
)

type sessionHandler = func(conn *websocket.Conn, info *server.RequestInfo, callback func()) error

// WebsocketHandler generate a new http handler for WebSocket connections
func WebsocketHandler(subprotocols []string, headersExtractor server.HeadersExtractor, config *Config, sessionHandler sessionHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := slog.With("context", "ws")

		upgrader := websocket.Upgrader{
			CheckOrigin:       CheckOrigin(config.AllowedOrigins),
			Subprotocols:      subprotocols,
			ReadBufferSize:    config.ReadBufferSize,
			WriteBufferSize:   config.WriteBufferSize,
			EnableCompression: config.EnableCompression,
		}

		rheader := map[string][]string{"X-AnyCable-Version": {version.Version()}}
		wsc, err := upgrader.Upgrade(w, r, rheader)
		if err != nil {
			ctx.Debug("WebSocket connection upgrade failed", "error", err.Error())
			return
		}

		info, err := server.NewRequestInfo(r, headersExtractor)
		if err != nil {
			CloseWithReason(wsc, websocket.CloseAbnormalClosure, err.Error())
			return
		}

		wsc.SetReadLimit(config.MaxMessageSize)

		if config.EnableCompression {
			wsc.EnableWriteCompression(true)
		}

		sessionCtx := slog.With("sid", info.UID)

		clientSubprotocol := r.Header.Get("Sec-Websocket-Protocol")

		if wsc.Subprotocol() == "" && clientSubprotocol != "" {
			sessionCtx.Debug("no subprotocol negotiated", "client", clientSubprotocol, "server", subprotocols)
		}

		// Separate goroutine for better GC of caller's data.
		go func() {
			sessionCtx.Debug("WebSocket session established")
			serr := sessionHandler(wsc, info, func() {
				sessionCtx.Debug("WebSocket session completed")
			})

			if serr != nil {
				sessionCtx.Error("WebSocket session failed", "error", serr)
				return
			}
		}()
	})
}

func CheckOrigin(origins string) func(r *http.Request) bool {
	if origins == "" {
		return func(r *http.Request) bool { return true }
	}

	hosts := strings.Split(strings.ToLower(origins), ",")

	return func(r *http.Request) bool {
		origin := strings.ToLower(r.Header.Get("Origin"))
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}

		for _, host := range hosts {
			if host[0] == '*' && strings.HasSuffix(u.Host, host[1:]) {
				return true
			}
			if u.Host == host {
				return true
			}
		}
		return false
	}
}
