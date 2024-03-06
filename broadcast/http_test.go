package broadcast

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

func TestHttpHandler(t *testing.T) {
	handler := &mocks.Handler{}
	config := NewHTTPConfig()

	secretConfig := NewHTTPConfig()
	secretConfig.SecretBase = "qwerty"
	broadcastKey := "42923a28b760e667fc92f7c6123bb07a282822b329dd2ef48e7aee7830d98485"

	broadcaster := NewHTTPBroadcaster(handler, &config, slog.Default())
	protectedBroadcaster := NewHTTPBroadcaster(handler, &secretConfig, slog.Default())

	done := make(chan (error))

	require.NoError(t, broadcaster.Start(done))
	defer broadcaster.Shutdown(context.Background()) // nolint:errcheck

	require.NoError(t, protectedBroadcaster.Start(done))
	defer protectedBroadcaster.Shutdown(context.Background()) // nolint:errcheck

	payload, err := json.Marshal(map[string]string{"stream": "any_test", "data": "123_test"})
	if err != nil {
		t.Fatal(err)
	}

	handler.On(
		"HandleBroadcast",
		payload,
	)

	t.Run("Handles broadcasts", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/", strings.NewReader(string(payload)))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(broadcaster.Handler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("Rejects non-POST requests", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/", strings.NewReader(string(payload)))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(broadcaster.Handler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
	})

	t.Run("Rejects when authorization header is missing", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/", strings.NewReader(string(payload)))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(protectedBroadcaster.Handler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("Accepts when authorization header is valid", func(t *testing.T) {
		req, err := http.NewRequest("POST", "/", strings.NewReader(string(payload)))
		req.Header.Set("Authorization", "Bearer "+broadcastKey)

		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(protectedBroadcaster.Handler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})
}
