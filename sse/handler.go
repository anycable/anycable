package sse

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/version"
	"github.com/anycable/anycable-go/ws"
	"github.com/apex/log"
)

// SSEHandler generates a new http handler for SSE connections
func SSEHandler(n *node.Node, headersExtractor server.HeadersExtractor, config *Config) http.Handler {
	allowedHosts := strings.Split(config.AllowedOrigins, ",")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write CORS headers
		if config.AllowedOrigins == "" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			origin := strings.ToLower(r.Header.Get("Origin"))
			u, err := url.Parse(origin)
			if err == nil {
				for _, host := range allowedHosts {
					if host[0] == '*' && strings.HasSuffix(u.Host, host[1:]) {
						w.Header().Set("Access-Control-Allow-Origin", origin)
					}
					if u.Host == host {
						w.Header().Set("Access-Control-Allow-Origin", origin)
					}
				}
			}
		}

		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, X-Request-ID, Content-Type, Accept, X-CSRF-Token, Authorization")

		// Respond to preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// SSE only supports GET requests
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Prepare common headers
		w.Header().Set("X-AnyCable-Version", version.Version())
		if r.ProtoMajor == 1 {
			// An endpoint MUST NOT generate an HTTP/2 message containing connection-specific header fields.
			// Source: RFC7540.
			w.Header().Set("Connection", "keep-alive")
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Accel-Buffering", "no")
		w.Header().Set("Cache-Control", "private, no-cache, no-store, must-revalidate, max-age=0") // HTTP 1.1
		w.Header().Set("Pragma", "no-cache")                                                       // HTTP 1.0
		w.Header().Set("Expire", "0")

		flusher, ok := w.(http.Flusher)
		if !ok {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}

		info, err := server.NewRequestInfo(r, headersExtractor)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		sessionCtx := log.WithField("sid", info.UID).WithField("transport", "sse")

		subscribeCmd, err := subscribeCommandFromRequest(r)

		if err != nil {
			sessionCtx.Errorf("failed to build subscribe command: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Handle subscription
		if subscribeCmd == nil {
			sessionCtx.Error("no channel provided")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Finally, we can establish a session
		session, err := NewSSESession(n, w, info)

		if err != nil {
			sessionCtx.Errorf("failed to establish sesssion: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if session == nil {
			sessionCtx.Error("authentication failed")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		conn := session.UnderlyingConn().(*Connection)

		// Subscribe to the channel
		res, err := n.Subscribe(session, subscribeCmd)

		if err != nil || res == nil {
			sessionCtx.Errorf("failed to subscribe: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Subscription rejected
		if res.Status != common.SUCCESS {
			sessionCtx.Debugf("rejected: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		conn.Established()
		sessionCtx.Debugf("session established")

		// TODO: Handle server shutdown. Currently, server is waiting for SSE connections to be closed
		select {
		case <-r.Context().Done():
			sessionCtx.Debugf("request terminated")
			session.DisconnectNow("Closed", ws.CloseNormalClosure)
			return
		case <-conn.Context().Done():
			sessionCtx.Debugf("session completed")
			return
		}
	})
}
