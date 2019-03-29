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
