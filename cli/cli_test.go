package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHeadersArg(t *testing.T) {
	config := Config{}
	config.parseHeaders("cookie,X-API-TOKEN,Origin")

	expected := []string{"cookie", "x-api-token", "origin"}

	assert.Equal(t, expected, config.Headers)
}

func TestLoadConfigDefaults(t *testing.T) {
	config := LoadConfig()
	assert.Equal(t, "0.0.0.0:50051", config.RPCHost)
	assert.Equal(t, "redis://localhost:6379/5", config.RedisURL)
	assert.Equal(t, "__anycable__", config.RedisChannel)
	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, 8080, config.Port)
	assert.Equal(t, "/cable", config.Path)
	assert.Equal(t, 100, config.DisconnectRate)
}

func TestLoadConfigSSL(t *testing.T) {
	config := LoadConfig()
	assert.False(t, config.SSL.Available())

	config.SSL.CertPath = "secret.cert"
	assert.False(t, config.SSL.Available())

	config.SSL.KeyPath = "secret.key"
	assert.True(t, config.SSL.Available())
}

func TestLoadConfigEnv(t *testing.T) {
	os.Setenv("ANYCABLE_PORT", "3334")
	os.Setenv("ANYCABLE_HOST", "localhost")
	os.Setenv("ANYCABLE_REDIS_URL", "redis://somewhere:6379/")

	config := LoadConfig()

	assert.Equal(t, 3334, config.Port)
	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, "redis://somewhere:6379/", config.RedisURL)

	os.Setenv("PORT", "5432")
	os.Setenv("REDIS_URL", "redis://redis:6379/")

	config2 := LoadConfig()

	assert.Equal(t, 5432, config2.Port)
	assert.Equal(t, "redis://redis:6379/", config2.RedisURL)
}
