package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogEnabled(t *testing.T) {
	config := NewConfig()
	assert.False(t, config.LogEnabled())

	config.Log = true
	assert.True(t, config.LogEnabled())

	config.Log = false
	assert.False(t, config.LogEnabled())

	config = NewConfig()
	config.LogFormatter = "test"
	assert.True(t, config.LogEnabled())

	config = NewConfig()
	config.LogInterval = 2
	assert.True(t, config.LogEnabled())
}

func TestHTTPEnabled(t *testing.T) {
	config := NewConfig()
	assert.False(t, config.HTTPEnabled())

	config.HTTP = "/metrics"
	assert.True(t, config.HTTPEnabled())
}
