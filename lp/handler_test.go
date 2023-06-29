package lp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLongPollingHandler(t *testing.T) {
	appNode := buildNode()
	conf := NewConfig()

	conf.FlushInterval = 10
	conf.KeepaliveTimeout = 1
	conf.PollInterval = 1

	dconfig := node.NewDisconnectQueueConfig()
	dconfig.Rate = 1
	disconnector := node.NewDisconnectQueue(appNode, &dconfig, slog.Default())
	appNode.SetDisconnector(disconnector)

	hub := NewHub(appNode, nil, &conf, slog.Default())
	go hub.Run()
	defer hub.Shutdown(context.Background()) // nolint: errcheck

	headersExtractor := &server.DefaultHeadersExtractor{}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/", nil)

	handler := LongPollingHandler(hub, context.Background(), headersExtractor, &conf, slog.Default())
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "private, no-cache, no-store, must-revalidate, max-age=0", w.Header().Get("Cache-Control"))
	assert.Equal(t, "no-cache", w.Header().Get("Pragma"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))

	require.Equal(t, "welcome\n", w.Body.String())

	id := hub.heap.Peek().Value().id
	session := hub.heap.Peek().Value().session

	// Check that session ID is returned in the header
	assert.Equal(t, id, w.Header().Get("X-AnyCable-Poll-ID"))

	t.Run("request with command", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/", nil)
		req.Header.Set("X-AnyCable-Poll-ID", id)

		// add request body as JSONL
		req.Body = io.NopCloser(
			strings.NewReader("{\"command\":\"subscribe\",\"identifier\":\"test\"}\n{\"command\":\"subscribe\",\"identifier\":\"test2\"}"),
		)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		// See mock_controller for transmissions logic — we should fix this :(
		assert.Equal(t, fmt.Sprintf("%s\n%s\n", session.GetID(), session.GetID()), w.Body.String())
	})

	t.Run("request without commands", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/", nil)
		req.Header.Set("X-AnyCable-Poll-ID", id)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, w.Body.String())
	})

	t.Run("when prev session expired", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/", nil)
		req.Header.Set("X-AnyCable-Poll-ID", id)

		// Mark session is disconnected
		session.Disconnect("Test", 1001)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, "{\"type\":\"disconnect\",\"reason\":\"session_expired\",\"reconnect\":true}", w.Body.String())
	})
}
