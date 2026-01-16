package ds

import (
	"context"
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
	"github.com/anycable/anycable-go/ws"
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

		// Authenticate and init stream
		stream, err := NewDSSession(n, w, info, streamParams)

		if err != nil {
			l.Warn("unauthenticated", "err", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		stream.Session.Log.Debug("session established", "mode", stream.Params.LiveMode, "offset", stream.Params.RawOffset)
		defer func() {
			stream.Session.Log.Debug("session completed")
			stream.Session.Disconnect("done", ws.CloseNormalClosure)
		}()

		// TODO: check that stream exists and get the tail offset
		// (same mechanism as in head)
		tail := &common.StreamMessage{}

		// TODO: Now we need to authorize access to the stream (signed streams?)

		if r.Method == http.MethodHead {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")

			w.Header().Set(StreamOffsetHeader, EncodeOffset(tail.Offset, tail.Epoch))

			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == http.MethodGet {
			if m != nil {
				m.CounterIncrement(metricsRequestsTotal)
			}

			if streamParams.LiveMode != SSEMode {
				handleHTTP(n, brk, config, m, w, r, stream, tail, shutdownCtx)
			} else {
				// TODO: implement SSE
				w.WriteHeader(http.StatusNotImplemented)
			}
			return
		}
	})
}

func handleHTTP(n *node.Node, brk broker.Broker, c *Config, m metrics.Instrumenter, w http.ResponseWriter, r *http.Request, s *Stream, tail *common.StreamMessage, shutdownCtx context.Context) {
	w.Header().Set("Content-Type", "application/json")
	// TODO: how to distinguish private streams?
	w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	w.Header().Set(StreamCursorHeader, s.NextCursor())

	backlog, err := fetchHistory(brk, s.Params)

	if err != nil {
		s.Session.Log.Error("failed to fetch history", "err", err)
		// Either offset is unreachable or epoch has changed
		w.WriteHeader(http.StatusGone)
		return
	}

	if len(backlog) > 0 {
		tail = &backlog[len(backlog)-1]
	}

	if len(backlog) > 0 || s.Params.LiveMode != LongPollMode {
		w.Header().Set(StreamOffsetHeader, EncodeOffset(tail.Offset, tail.Epoch))
		w.Header().Set(StreamUpToDateHeader, "true")

		w.Write(EncodeJSONBatch(backlog)) // nolint: errcheck
		return
	}

	if m != nil {
		m.CounterIncrement(metricsPollTotal)
		m.GaugeIncrement(metricsPollNum)
		defer m.GaugeDecrement(metricsPollNum)
	}

	_, err = n.Subscribe(s.Session, s.Params.ToSubscribeCommand())

	if err != nil {
		s.Session.Log.Error("failed to subscribe", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	conn := s.Conn.(*PollConnection)
	disconnectNotify := r.Context().Done()
	flushNotify := conn.Context().Done()
	timeout := c.PollInterval

	shutdownReceived := false

	for {
		select {
		case <-shutdownCtx.Done():
			if !shutdownReceived {
				shutdownReceived = true
				s.Session.Log.Debug("server shutdown")
				conn.Close(http.StatusGone, "Server shutdown")
			}
		case <-time.After(time.Duration(timeout) * time.Second):
			conn.Close(http.StatusNoContent, "No content")
			return
		case <-disconnectNotify:
			s.Session.Log.Debug("polling request terminated")
			return
		case <-flushNotify:
			s.Session.Log.Debug("polling request fulfilled")
			return
		}
	}
}

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
