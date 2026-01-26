package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anycable/anycable-go/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishHandler(t *testing.T) {
	payload, err := json.Marshal(map[string]string{"stream": "test_stream", "data": "test_data"})
	require.NoError(t, err)

	t.Run("Handles broadcasts without auth", func(t *testing.T) {
		handler := mocks.NewHandler(t)
		config := NewConfig()

		server, err := NewServer(&config, handler, slog.Default())
		require.NoError(t, err)
		defer server.Shutdown(context.Background()) // nolint:errcheck

		handler.On("HandleBroadcast", payload).Return(nil)

		req, err := http.NewRequest("POST", "/api/publish", strings.NewReader(string(payload)))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		http.HandlerFunc(server.PublishHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("Rejects non-POST requests", func(t *testing.T) {
		handler := mocks.NewHandler(t)
		config := NewConfig()

		server, err := NewServer(&config, handler, slog.Default())
		require.NoError(t, err)
		defer server.Shutdown(context.Background()) // nolint:errcheck

		req, err := http.NewRequest("GET", "/api/publish", strings.NewReader(string(payload)))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		http.HandlerFunc(server.PublishHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
	})

	t.Run("Accepts when authorization header is valid", func(t *testing.T) {
		handler := mocks.NewHandler(t)
		config := NewConfig()
		config.Secret = "test-secret"

		server, err := NewServer(&config, handler, slog.Default())
		require.NoError(t, err)
		defer server.Shutdown(context.Background()) // nolint:errcheck

		handler.On("HandleBroadcast", payload).Return(nil)

		req, err := http.NewRequest("POST", "/api/publish", strings.NewReader(string(payload)))
		req.Header.Set("Authorization", "Bearer test-secret")
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		http.HandlerFunc(server.PublishHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("Handles CORS preflight requests", func(t *testing.T) {
		handler := mocks.NewHandler(t)
		config := NewConfig()
		config.AddCORSHeaders = true

		server, err := NewServer(&config, handler, slog.Default())
		require.NoError(t, err)
		defer server.Shutdown(context.Background()) // nolint:errcheck

		req, err := http.NewRequest("OPTIONS", "/api/publish", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		http.HandlerFunc(server.PublishHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NotEmpty(t, rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("Includes CORS headers on POST response", func(t *testing.T) {
		handler := mocks.NewHandler(t)
		config := NewConfig()
		config.AddCORSHeaders = true

		server, err := NewServer(&config, handler, slog.Default())
		require.NoError(t, err)
		defer server.Shutdown(context.Background()) // nolint:errcheck

		handler.On("HandleBroadcast", payload).Return(nil)

		req, err := http.NewRequest("POST", "/api/publish", strings.NewReader(string(payload)))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		http.HandlerFunc(server.PublishHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.NotEmpty(t, rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("CORS with specific hosts", func(t *testing.T) {
		handler := mocks.NewHandler(t)
		config := NewConfig()
		config.AddCORSHeaders = true
		config.CORSHosts = "example.com,test.com"

		server, err := NewServer(&config, handler, slog.Default())
		require.NoError(t, err)
		defer server.Shutdown(context.Background()) // nolint:errcheck

		handler.On("HandleBroadcast", payload).Return(nil)

		req, err := http.NewRequest("POST", "/api/publish", strings.NewReader(string(payload)))
		req.Header.Set("Origin", "http://example.com")
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		http.HandlerFunc(server.PublishHandler).ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, "http://example.com", rr.Header().Get("Access-Control-Allow-Origin"))
	})
}
