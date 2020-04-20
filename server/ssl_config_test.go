package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSSLAvailable(t *testing.T) {
	config := NewSSLConfig()
	assert.False(t, config.Available())

	config.CertPath = "secret.cert"
	assert.False(t, config.Available())

	config.KeyPath = "secret.key"
	assert.True(t, config.Available())
}
