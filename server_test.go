package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHeadersArg(t *testing.T) {
	parsed := ParseHeadersArg("cookie,X-API-TOKEN,Origin")
	expected := []string{"cookie", "x-api-token", "origin"}

	assert.Equal(t, expected, parsed)
}
