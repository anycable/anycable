package server

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeadersExtractor_FromRequest(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Cookie", "yummy_cookie=raisin;tasty_cookie=strawberry")
	req.Header.Set("X-Api-Token", "42")
	req.Header.Set("Accept-Language", "ru")

	t.Run("Without specified headers", func(t *testing.T) {
		extractor := DefaultHeadersExtractor{}
		headers := extractor.FromRequest(req)

		assert.Len(t, headers, 1)
		assert.Equal(t, "192.0.2.1", headers["REMOTE_ADDR"])
	})

	t.Run("With specified headers", func(t *testing.T) {
		extractor := DefaultHeadersExtractor{Headers: []string{"cookie", "x-api-token", "x-jid"}}
		headers := extractor.FromRequest(req)

		assert.Len(t, headers, 3)

		assert.Empty(t, headers["accept-language"])
		assert.Equal(t, "42", headers["x-api-token"])
		assert.Equal(t, "yummy_cookie=raisin;tasty_cookie=strawberry", headers["cookie"])
		assert.Equal(t, "192.0.2.1", headers["REMOTE_ADDR"])

		_, ok := headers["x-jid"]
		assert.False(t, ok)
	})

	t.Run("With specified headers and cookie filter", func(t *testing.T) {
		extractor := DefaultHeadersExtractor{Headers: []string{"cookie"}, Cookies: []string{"yummy_cookie"}}
		headers := extractor.FromRequest(req)

		assert.Len(t, headers, 2)

		assert.Empty(t, headers["accept-language"])
		assert.Equal(t, "yummy_cookie=raisin;", headers["cookie"])
		assert.Equal(t, "192.0.2.1", headers["REMOTE_ADDR"])
	})

	t.Run("With specified auth header", func(t *testing.T) {
		extractor := DefaultHeadersExtractor{AuthHeader: "x-api-token", Headers: []string{"cookie", "x-jid"}}
		headers := extractor.FromRequest(req)

		assert.Len(t, headers, 3)

		assert.Empty(t, headers["accept-language"])
		assert.Equal(t, "42", headers["x-api-token"])
		assert.Equal(t, "yummy_cookie=raisin;tasty_cookie=strawberry", headers["cookie"])
		assert.Equal(t, "192.0.2.1", headers["REMOTE_ADDR"])

		_, ok := headers["x-jid"]
		assert.False(t, ok)
	})
}
