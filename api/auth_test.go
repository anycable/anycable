package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthenticator(t *testing.T) {
	t.Run("NewAuthenticator with explicit secret", func(t *testing.T) {
		auth, err := NewAuthenticator("my-secret")
		require.NoError(t, err)

		assert.True(t, auth.IsEnabled())
		assert.Equal(t, "my-secret", auth.Secret())
	})

	t.Run("Authenticate with no secret configured (public)", func(t *testing.T) {
		auth, err := NewAuthenticator("")
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/api/publish", nil)
		assert.True(t, auth.Authenticate(req))
	})

	t.Run("Authenticate with valid Bearer token", func(t *testing.T) {
		auth, err := NewAuthenticator("test-secret")
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/api/publish", nil)
		req.Header.Set("Authorization", "Bearer test-secret")

		assert.True(t, auth.Authenticate(req))
	})

	t.Run("Authenticate rejects missing Authorization header", func(t *testing.T) {
		auth, err := NewAuthenticator("test-secret")
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/api/publish", nil)

		assert.False(t, auth.Authenticate(req))
	})

	t.Run("Authenticate rejects invalid token", func(t *testing.T) {
		auth, err := NewAuthenticator("test-secret")
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/api/publish", nil)
		req.Header.Set("Authorization", "Bearer wrong-secret")

		assert.False(t, auth.Authenticate(req))
	})

	t.Run("Authenticate rejects non-Bearer auth", func(t *testing.T) {
		auth, err := NewAuthenticator("test-secret")
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/api/publish", nil)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

		assert.False(t, auth.Authenticate(req))
	})
}

func TestAuthenticator_Integration(t *testing.T) {
	auth, err := NewAuthenticator("my-secret")
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/api/publish", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer my-secret")

	assert.True(t, auth.Authenticate(req))
}
