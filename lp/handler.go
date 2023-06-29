package lp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/version"
)

const (
	pollIdHeader = "x-anycable-poll-id"
	pollIdCookie = "__anypoll_sid"
)

// LongPollingHandler generates a new http handler for long-polling connections
func LongPollingHandler(hub *Hub, shutdownCtx context.Context, headersExtractor server.HeadersExtractor, config *Config, l *slog.Logger) http.Handler {
	maxBytesSize := config.MaxBodySize
	if maxBytesSize == 0 {
		maxBytesSize = defaultMaxBodySize
	}

	var allowedHosts []string

	if config.AllowedOrigins == "" {
		allowedHosts = []string{}
	} else {
		allowedHosts = strings.Split(config.AllowedOrigins, ",")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write CORS headers
		server.WriteCORSHeaders(w, r, allowedHosts)

		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, X-Request-ID, Content-Type, Accept, X-CSRF-Token, Authorization, X-Anycable-Poll-Id")
		w.Header().Set("Access-Control-Expose-Headers", "X-Anycable-Poll-Id")

		// Respond to preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// We only support POST requests
		if r.Method != http.MethodPost {
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
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Accel-Buffering", "no")
		w.Header().Set("Cache-Control", "private, no-cache, no-store, must-revalidate, max-age=0") // HTTP 1.1
		w.Header().Set("Pragma", "no-cache")                                                       // HTTP 1.0
		w.Header().Set("Expire", "0")                                                              // Proxies

		var session *node.Session
		var err error

		info, err := server.NewRequestInfo(r, headersExtractor)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		sessionCtx := l.With("sid", info.UID)
		pollId := r.Header.Get(pollIdHeader)

		if pollId == "" {
			pollId, session, err = hub.NewSession(w, info)
		} else {
			session, err = hub.FindSession(w, pollId)
		}

		if err != nil {
			sessionCtx.Error("long polling session failed", "error", err)
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		// No need to handle anything, nil means that authentication failed
		if session == nil || pollId == "" {
			return
		}

		sessionCtx.Debug("long polling session established")

		w.Header().Set(pollIdHeader, pollId)

		conn := session.UnderlyingConn().(*Connection)
		defer hub.Disconnected(pollId)

		// Read commands (if any)
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytesSize))
			requestData, err := io.ReadAll(r.Body)

			if err != nil {
				sessionCtx.Error("error reading body", "error", err)
				if len(requestData) >= maxBytesSize {
					conn.Close(http.StatusRequestEntityTooLarge, "Request entity too large")
					return
				}
				conn.Close(http.StatusInternalServerError, "Internal server error")
				return
			}

			if len(requestData) > 0 {
				lines := bytes.Split(requestData, []byte("\n"))

				for _, line := range lines {
					if len(line) > 0 {
						var command json.RawMessage
						err := json.Unmarshal(line, &command)

						if err != nil {
							sessionCtx.Error("long polling session failed to process request", "error", err)
							conn.Close(http.StatusBadRequest, "Bad request")
							return
						}

						session.ReadMessage(command) // nolint: errcheck
					}
				}
			}
		}

		disconnectNotify := r.Context().Done()
		flushNotify := conn.Context().Done()
		timeout := config.PollInterval

		conn.Flush()

		shutdownReceived := false

		defer func() {
			sessionCtx.Debug("long polling session completed")
		}()

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
			case <-time.After(time.Duration(timeout) * time.Second):
				conn.Close(http.StatusNoContent, "No content")
				return
			case <-disconnectNotify:
				sessionCtx.Debug("long polling request terminated")
				conn.Closed()
				return
			case <-flushNotify:
				sessionCtx.Debug("long polling request fulfilled")
				return
			}
		}
	})
}
