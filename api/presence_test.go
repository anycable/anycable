package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPresenceHandler(t *testing.T) {
	t.Run("Rejects non-GET requests", func(t *testing.T) {
		handler := mocks.NewHandler(t)
		brk := mocks.NewBroker(t)

		config := NewConfig()

		api, err := NewServer(&config, brk, handler, slog.Default())
		require.NoError(t, err)
		require.NoError(t, api.Start())
		defer api.Shutdown(context.Background()) // nolint:errcheck

		req, err := http.NewRequest("POST", "/api/presence/a/users", strings.NewReader(""))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		api.server.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
	})

	t.Run("Returns presence info", func(t *testing.T) {
		handler := mocks.NewHandler(t)
		brk := mocks.NewBroker(t)
		brk.On("SupportsPresence").Return(true)

		brk.On("PresenceInfo", "a", mock.Anything).Return(
			&common.PresenceInfo{
				Total: 2,
				Records: []*common.PresenceEvent{
					{ID: "33", Info: map[string]string{"name": "jack"}},
					{ID: "44", Info: map[string]string{"name": "jill"}},
				},
			},
			nil,
		)

		config := NewConfig()
		config.Secret = "test-secret"

		api, err := NewServer(&config, brk, handler, slog.Default())
		require.NoError(t, err)
		require.NoError(t, api.Start())
		defer api.Shutdown(context.Background()) // nolint:errcheck

		req, err := http.NewRequest("GET", "/api/presence/a/users", nil)
		req.Header.Set("Authorization", "Bearer test-secret")
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		api.server.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		body := rr.Body.String()

		var parsed struct {
			Total   int `json:"total"`
			Records []struct {
				ID   string `json:"id"`
				Info struct {
					Name string `json:"name"`
				} `json:"info"`
			} `json:"records"`
		}
		err = json.Unmarshal([]byte(body), &parsed)
		require.NoError(t, err)

		assert.Equal(t, 2, parsed.Total)

		require.Len(t, parsed.Records, 2)
		assert.Equal(t, "33", parsed.Records[0].ID)
		assert.Equal(t, "jack", parsed.Records[0].Info.Name)
		assert.Equal(t, "44", parsed.Records[1].ID)
		assert.Equal(t, "jill", parsed.Records[1].Info.Name)
	})

	t.Run("Returns empty presence info when stream has no presence information", func(t *testing.T) {
		handler := mocks.NewHandler(t)
		brk := mocks.NewBroker(t)

		brk.On("SupportsPresence").Return(true)

		brk.On("PresenceInfo", "b", mock.Anything).Return(
			&common.PresenceInfo{},
			nil,
		)

		config := NewConfig()
		api, err := NewServer(&config, brk, handler, slog.Default())
		require.NoError(t, err)
		require.NoError(t, api.Start())
		defer api.Shutdown(context.Background()) // nolint:errcheck

		req, err := http.NewRequest("GET", "/api/presence/b/users", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		api.server.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		body := rr.Body.String()

		var parsed map[string]interface{}
		err = json.Unmarshal([]byte(body), &parsed)
		require.NoError(t, err)

		assert.Equal(t, 0.0, parsed["total"])

		_, ok := parsed["records"]
		require.False(t, ok)
	})

	t.Run("Returns not implemented when presence is not supported", func(t *testing.T) {
		handler := mocks.NewHandler(t)
		brk := mocks.NewBroker(t)

		brk.On("SupportsPresence").Return(false)

		config := NewConfig()
		api, err := NewServer(&config, brk, handler, slog.Default())
		require.NoError(t, err)
		require.NoError(t, api.Start())
		defer api.Shutdown(context.Background()) // nolint:errcheck

		req, err := http.NewRequest("GET", "/api/presence/b/users", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		api.server.ServeHTTP(rr, req)

		require.Equal(t, http.StatusNotImplemented, rr.Code)
	})
}
