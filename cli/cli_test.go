package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHeadersArg(t *testing.T) {
	expected := []string{"cookie", "x-api-token", "origin"}

	headers := parseHeaders("cookie,X-API-TOKEN,Origin")
	assert.Equal(t, expected, headers)
}
