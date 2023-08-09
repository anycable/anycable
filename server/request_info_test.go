package server

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchUID(t *testing.T) {
	t.Run("Without request id", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		uid, _ := FetchUID(req)

		assert.NotEqual(t, "", uid)
	})

	t.Run("With request id", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("x-request-id", "external-request-id")

		uid, _ := FetchUID(req)

		assert.Equal(t, "external-request-id", uid)
	})
}

func TestRequestInfo(t *testing.T) {
	t.Run("With just path", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/cable", nil)
		info, err := NewRequestInfo(req, nil)

		require.NoError(t, err)
		assert.Equal(t, "http://example.com/cable", info.URL)
	})

	t.Run("With just path + TLS", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/cable", nil)
		req.TLS = &tls.ConnectionState{}
		info, err := NewRequestInfo(req, nil)

		require.NoError(t, err)
		assert.Equal(t, "https://example.com/cable", info.URL)
	})

	t.Run("With fully qualified URL", func(t *testing.T) {
		req := httptest.NewRequest("GET", "ws://anycable.io/cable", nil)
		info, err := NewRequestInfo(req, nil)

		require.NoError(t, err)
		assert.Equal(t, "ws://anycable.io/cable", info.URL)
	})

	t.Run("With params", func(t *testing.T) {
		req := httptest.NewRequest("GET", "ws://anycable.io/cable?pi=3&pp=no&pi=5", nil)
		info, err := NewRequestInfo(req, nil)

		require.NoError(t, err)
		assert.Equal(t, "5", info.Param("pi"))
		assert.Equal(t, "no", info.Param("pp"))

		blank_info := RequestInfo{}
		assert.Equal(t, "", blank_info.Param("pi"))
	})
}
