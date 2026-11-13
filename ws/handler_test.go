package ws

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func TestCheckOriginWildcards(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	req.Header.Set("Origin", "https://evilexample.com")
	assert.False(t, CheckOrigin("*example.com")(req))
	assert.False(t, CheckOrigin("*.example.com")(req))
	assert.True(t, CheckOrigin("*")(req))

	req.Header.Set("Origin", "https://example.com")
	assert.True(t, CheckOrigin("*example.com")(req))
	assert.False(t, CheckOrigin("*.example.com")(req))

	req.Header.Set("Origin", "https://app.example.com")
	assert.True(t, CheckOrigin("*example.com")(req))
	assert.True(t, CheckOrigin("*.example.com")(req))
}

func TestCheckOriginEmptyEntries(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")

	assert.True(t, CheckOrigin("example.com,")(req))
	assert.False(t, CheckOrigin("other.com, ,")(req))
}
