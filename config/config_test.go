package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSSLAvailable(t *testing.T) {
	config := New()
	assert.False(t, config.SSL.Available())

	config.SSL.CertPath = "secret.cert"
	assert.False(t, config.SSL.Available())

	config.SSL.KeyPath = "secret.key"
	assert.True(t, config.SSL.Available())
}
