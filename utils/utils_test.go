package utils

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestFetchHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Cookies", "yummy_cookie=raisin; tasty_cookie=strawberry")
	req.Header.Set("X-Api-Token", "42")
	req.Header.Set("Accept-Language", "ru")

	t.Run("Without specified headers ", func(t *testing.T) {
		headers := FetchHeaders(req, []string{})

		assert.Len(t, headers, 1)
		assert.Equal(t, "192.0.2.1", headers["REMOTE_ADDR"])
	})

	t.Run("With specified headers ", func(t *testing.T) {
		headers := FetchHeaders(req, []string{"cookies", "x-api-token"})

		assert.Len(t, headers, 3)

		assert.Empty(t, headers["accept-language"])
		assert.Equal(t, req.Header.Get("x-api-token"), headers["x-api-token"])
		assert.Equal(t, req.Header.Get("cookies"), headers["cookies"])
		assert.Equal(t, "192.0.2.1", headers["REMOTE_ADDR"])
	})
}
