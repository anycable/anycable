package sse

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/version"
	"github.com/anycable/anycable-go/ws"
)

// SSEHandler generates a new http handler for SSE connections
func SSEHandler(n *node.Node, m metrics.Instrumenter, shutdownCtx context.Context, headersExtractor server.HeadersExtractor, config *Config, l *slog.Logger) http.Handler {
	var allowedHosts []string

	if config.AllowedOrigins == "" {
		allowedHosts = []string{}
	} else {
		allowedHosts = strings.Split(config.AllowedOrigins, ",")
	}

	if m != nil {
		m.RegisterGauge(metricsClientsNum, "The number of active SSE clients")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write CORS headers
		server.WriteCORSHeaders(w, r, allowedHosts)

		// Respond to preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// SSE only supports GET and POST requests
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
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

		sessionCtx := l.With("sid", info.UID).With("transport", "sse")

		subscribeCmds, err := subscribeCommandsFromRequest(r)

		if err != nil {
			sessionCtx.Error("failed to build subscribe command", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Finally, we can establish a session
		session, err := NewSSESession(n, w, r, info)

		if err != nil {
			sessionCtx.Error("failed to establish sesssion", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if session == nil {
			sessionCtx.Error("authentication failed")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Make sure we remove the session from the node when we're done (especially if we return earlier due to rejected subscription)
		defer session.Disconnect("Closed", ws.CloseNormalClosure)

		conn := session.UnderlyingConn().(*Connection)

		for _, subscribeCmd := range subscribeCmds {
			// Subscribe to the channel
			res, err := n.Subscribe(session, subscribeCmd)

			if err != nil || res == nil {
				sessionCtx.Error("failed to subscribe", "error", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// Subscription rejected
			if res.Status != common.SUCCESS {
				sessionCtx.Debug("subscription rejected")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		conn.Established()
		sessionCtx.Debug("session established")

		shutdownReceived := false

		if m != nil {
			m.GaugeIncrement(metricsClientsNum)
			defer func() {
				m.GaugeDecrement(metricsClientsNum)
			}()
		}

		for {
			select {
			case <-shutdownCtx.Done():
				if !shutdownReceived {
					shutdownReceived = true
					sessionCtx.Debug("server shutdown")
					session.DisconnectWithMessage(
						&common.DisconnectMessage{Type: "disconnect", Reason: common.SERVER_RESTART_REASON, Reconnect: true},
						common.SERVER_RESTART_REASON,
					)
				}
			case <-r.Context().Done():
				sessionCtx.Debug("request terminated")
				session.DisconnectNow("Closed", ws.CloseNormalClosure)
				return
			case <-conn.Context().Done():
				sessionCtx.Debug("session completed")
				return
			}
		}
	})
}
