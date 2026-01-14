package ds

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/version"
)

const (
	defaultLongPollTimeout = 30 * time.Second
	defaultMaxMessages     = 100
)

// DSHandler generates a new http handler for Durable Streams connections
func DSHandler(n *node.Node, brk broker.Broker, m metrics.Instrumenter, shutdownCtx context.Context, headersExtractor server.HeadersExtractor, config *Config, l *slog.Logger) http.Handler {
	var allowedHosts []string

	if config.AllowedOrigins == "" {
		allowedHosts = []string{}
	} else {
		allowedHosts = strings.Split(config.AllowedOrigins, ",")
	}

	if m != nil {
		m.RegisterCounter(metricsRequestsTotal, "The total number of durable streams HEAD/GET requests")
		m.RegisterCounter(metricsPollTotal, "The total number of durable streams long-poll requests")
		m.RegisterCounter(metricsSSETotal, "The total number of durable streams SSE requests")
		m.RegisterGauge(metricsPollNum, "The number of active durable streams long-poll clients")
		m.RegisterGauge(metricsSSENum, "The number of active durable streams SSE clients")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write CORS headers
		server.WriteCORSHeaders(w, r, allowedHosts)

		// Respond to preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.Header().Set("X-AnyCable-Version", version.Version())

		streamParams, err := StreamParamsFromReq(r, config)
		if err != nil {
			l.With("transport", "ds").Debug("invalid stream request", "err", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// DS only supports GET and HEAD requests for reading data
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			l.With("transport", "ds").Debug("non-read operations are not supported")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		info, err := server.NewRequestInfo(r, headersExtractor)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		sessionCtx := l.With("sid", info.UID).With("transport", "ds").With("stream", streamParams.Path)
		session, err := NewDSSession(n, w, info, streamParams)

		if session == nil && err == nil {
			sessionCtx.Debug("authentication canceled")
			// TODO: which status?
			w.WriteHeader(http.StatusGone)
			return
		}

		if err != nil {
			sessionCtx.Error("failed to authenticate", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !session.IsConnected() {
			sessionCtx.Debug("unauthenticated", "err", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Authentication is passed at this point
		//
		// TODO: Now we need to authorize access to the stream

		if r.Method == http.MethodHead {
			handleHead(brk, w, streamParams, sessionCtx)
			return
		}

		if r.Method == http.MethodGet {
			if m != nil {
				m.CounterIncrement(metricsRequestsTotal)
			}

			if streamParams.LiveMode == "" {
				handleCatchup(brk, w, streamParams, sessionCtx)
			}
			return
		}
	})
}

func handleHead(brk broker.Broker, w http.ResponseWriter, sp *StreamParams, l *slog.Logger) {
	// TODO: we currently only support JSON streams
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	// FIXME: we must extend broker interface to allow getting a stream info
	// For now, we just return "now" as the next offset
	w.Header().Set(StreamOffsetHeader, "now")

	w.WriteHeader(http.StatusOK)
}

func handleCatchup(brk broker.Broker, w http.ResponseWriter, sp *StreamParams, l *slog.Logger) {
	// Set headers for catch-up mode
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")

	w.Header().Set(StreamCursorHeader, GenerateCursor(sp.Cursor))

	l.Debug("fetch history", "stream", sp.Path, "offset", sp.RawOffset)

	messages, err := fetchHistory(brk, sp)
	if err != nil {
		l.Debug("failed to fetch history", "error", err)
		w.WriteHeader(http.StatusGone)
		return
	}

	// Limit messages
	if len(messages) > defaultMaxMessages {
		messages = messages[:defaultMaxMessages]
	}

	// Encode messages as JSON array
	var jsonMessages []interface{}
	for _, msg := range messages {
		var data interface{}
		if jerr := json.Unmarshal([]byte(msg.Data), &data); jerr == nil {
			jsonMessages = append(jsonMessages, data)
		}
	}

	if jsonMessages == nil {
		jsonMessages = []interface{}{}
	}

	responseData, err := json.Marshal(jsonMessages)
	if err != nil {
		l.Error("failed to encode messages", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Set offset header
	var nextOffset uint64
	var nextEpoch string

	if len(messages) > 0 {
		lastMsg := messages[len(messages)-1]
		nextOffset = lastMsg.Offset
		nextEpoch = lastMsg.Epoch
	} else {
		nextOffset = sp.Offset
		nextEpoch = sp.Epoch
	}

	w.Header().Set(StreamOffsetHeader, EncodeOffset(nextOffset, nextEpoch))

	if len(messages) < defaultMaxMessages {
		w.Header().Set(StreamUpToDateHeader, "true")
	}

	w.WriteHeader(http.StatusOK)
	w.Write(responseData) // nolint: errcheck
}

// func handleLongPoll(w http.ResponseWriter, r *http.Request, streamPath string, offset uint64, epoch string, cursor string, brk broker.Broker, shutdownCtx context.Context, l *slog.Logger) {
// 	// Set headers for long-poll mode
// 	if r.ProtoMajor == 1 {
// 		w.Header().Set("Connection", "keep-alive")
// 	}
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Header().Set("Cache-Control", "private, max-age=60, stale-while-revalidate=300")

// 	// TODO: Implement actual long-polling with timeout
// 	// For now, we just do a catch-up read
// 	handleCatchup(w, streamPath, offset, epoch, cursor, brk, l)
// }

// func handleSSE(w http.ResponseWriter, r *http.Request, streamPath string, offset uint64, epoch string, cursor string, n *node.Node, brk broker.Broker, session *node.Session, conn *Connection, shutdownCtx context.Context, l *slog.Logger) {
// 	// Set headers for SSE mode
// 	if r.ProtoMajor == 1 {
// 		w.Header().Set("Connection", "keep-alive")
// 	}
// 	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
// 	w.Header().Set("X-Content-Type-Options", "nosniff")
// 	w.Header().Set("X-Accel-Buffering", "no")
// 	w.Header().Set("Cache-Control", "private, no-cache, no-store, must-revalidate, max-age=0")
// 	w.Header().Set("Pragma", "no-cache")
// 	w.Header().Set("Expire", "0")

// 	flusher, ok := w.(http.Flusher)
// 	if !ok {
// 		w.WriteHeader(http.StatusNotImplemented)
// 		return
// 	}

// 	// Fetch initial messages
// 	messages, err := fetchHistory(brk, streamPath, offset, epoch)
// 	if err != nil {
// 		l.Debug("failed to fetch history", "error", err)
// 		messages = []common.StreamMessage{}
// 	}

// 	w.WriteHeader(http.StatusOK)
// 	flusher.Flush()

// 	conn.Established()

// 	// Set encoder with cursor
// 	encoder := &Encoder{Cursor: cursor}
// 	session.SetEncoder(encoder)

// 	// Send initial messages
// 	for _, msg := range messages {
// 		reply := msg.ToReplyFor("")
// 		frame, err := encoder.Encode(reply)
// 		if err == nil && frame != nil {
// 			conn.Write(frame.Payload, time.Time{}) // nolint: errcheck
// 		}
// 	}

// 	// TODO: Subscribe to stream and forward new messages
// 	// For now, we just keep the connection open for a while

// 	l.Debug("SSE session established")

// 	// Keep connection alive
// 	ticker := time.NewTicker(30 * time.Second)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-shutdownCtx.Done():
// 			l.Debug("server shutdown")
// 			session.DisconnectNow("Closed", ws.CloseNormalClosure)
// 			return
// 		case <-r.Context().Done():
// 			l.Debug("request terminated")
// 			session.DisconnectNow("Closed", ws.CloseNormalClosure)
// 			return
// 		case <-conn.Context().Done():
// 			l.Debug("session completed")
// 			return
// 		case <-ticker.C:
// 			// Send a comment to keep connection alive
// 			w.Write([]byte(": keepalive\n\n")) // nolint: errcheck
// 			flusher.Flush()
// 		}
// 	}
// }

// fetchHistory retrieves history from broker
// If epoch is empty (initial read), uses HistorySince with timestamp 0 to get all history
// Otherwise uses HistoryFrom with the provided epoch and offset
func fetchHistory(brk broker.Broker, sp *StreamParams) ([]common.StreamMessage, error) {
	// Check reserved offsets first
	if sp.RawOffset == StartOffset || sp.RawOffset == "0" {
		return brk.HistorySince(sp.Path, 0)
	}

	if sp.RawOffset == "now" {
		return []common.StreamMessage{}, nil
	}

	if sp.Epoch == "" {
		return nil, errors.New("no epoch in the offset")
	}

	history, err := brk.HistoryFrom(sp.Path, sp.Epoch, sp.Offset)

	return history, err
}
