package ws

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

func TestHeadersExtractor_FromRequest(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "yummy_cookie=raisin;tasty_cookie=strawberry")
	req.Header.Set("X-Api-Token", "42")
	req.Header.Set("Accept-Language", "ru")

	t.Run("Without specified headers ", func(t *testing.T) {
		extractor := HeadersExtractor{}
		headers := extractor.FromRequest(req)

		assert.Len(t, headers, 1)
		assert.Equal(t, "192.0.2.1", headers["REMOTE_ADDR"])
	})

	t.Run("With specified headers ", func(t *testing.T) {
		extractor := HeadersExtractor{Headers: []string{"cookie", "x-api-token"}}
		headers := extractor.FromRequest(req)

		assert.Len(t, headers, 3)

		assert.Empty(t, headers["accept-language"])
		assert.Equal(t, req.Header.Get("x-api-token"), headers["x-api-token"])
		assert.Equal(t, req.Header.Get("cookie"), headers["cookie"])
		assert.Equal(t, "192.0.2.1", headers["REMOTE_ADDR"])
	})

	t.Run("With specified headers and cookie filter", func(t *testing.T) {
		extractor := HeadersExtractor{Headers: []string{"cookie"}, Cookies: []string{"yummy_cookie"}}
		headers := extractor.FromRequest(req)

		assert.Len(t, headers, 2)

		assert.Empty(t, headers["accept-language"])
		assert.Equal(t, "yummy_cookie=raisin;", headers["cookie"])
		assert.Equal(t, "192.0.2.1", headers["REMOTE_ADDR"])
	})
}

func TestCheckOriginWithoutHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	allowedOrigins := ""
	assert.Equal(t, CheckOrigin(allowedOrigins)(req), true)

	allowedOrigins = "secure.origin"
	assert.Equal(t, CheckOrigin(allowedOrigins)(req), false)
}

func TestCheckOrigin(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://my.localhost:8080")

	allowedOrigins := ""
	assert.Equal(t, CheckOrigin(allowedOrigins)(req), true)

	allowedOrigins = "my.localhost:8080"
	assert.Equal(t, CheckOrigin(allowedOrigins)(req), true)

	allowedOrigins = "MY.localhost:8080"
	assert.Equal(t, CheckOrigin(allowedOrigins)(req), true)

	allowedOrigins = "localhost:8080"
	assert.Equal(t, CheckOrigin(allowedOrigins)(req), false)

	allowedOrigins = "*.localhost:8080"
	assert.Equal(t, CheckOrigin(allowedOrigins)(req), true)

	allowedOrigins = "secure.origin,my.localhost:8080"
	assert.Equal(t, CheckOrigin(allowedOrigins)(req), true)

	allowedOrigins = "secure.origin,*.localhost:8080"
	assert.Equal(t, CheckOrigin(allowedOrigins)(req), true)

	req.Header.Set("Origin", "http://MY.localhost:8080")
	assert.Equal(t, CheckOrigin(allowedOrigins)(req), true)
}
