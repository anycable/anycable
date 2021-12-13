package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTagsArg(t *testing.T) {
	expected := map[string]string{"env": "dev", "rev": "1.1"}

	headers := parseTags("env:dev,rev:1.1")
	assert.Equal(t, expected, headers)
}
