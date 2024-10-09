package server

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig__DecodeToml(t *testing.T) {
	tomlString := `
  host = "0.0.0.0"
  port = 8081
  max_conn = 100
  health_path = "/healthz"
 `

	conf := NewConfig()
	_, err := toml.Decode(tomlString, &conf)
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", conf.Host)
	assert.Equal(t, 8081, conf.Port)
	assert.Equal(t, 100, conf.MaxConn)
	assert.Equal(t, "/healthz", conf.HealthPath)
}

func TestConfig__ToToml(t *testing.T) {
	conf := NewConfig()
	conf.Host = "local.test"
	conf.Port = 8082
	conf.HealthPath = "/healthz"
	conf.SSL.CertPath = "/path/to/cert"
	conf.SSL.KeyPath = "/path/to/key"
	conf.AllowedOrigins = "http://example.com"

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "host = \"local.test\"")
	assert.Contains(t, tomlStr, "port = 8082")
	assert.Contains(t, tomlStr, "# max_conn = 1000")
	assert.Contains(t, tomlStr, "health_path = \"/healthz\"")
	assert.Contains(t, tomlStr, "allowed_origins = \"http://example.com\"")
	assert.Contains(t, tomlStr, "ssl.cert_path = \"/path/to/cert\"")
	assert.Contains(t, tomlStr, "ssl.key_path = \"/path/to/key\"")

	// Round-trip test
	conf2 := Config{}

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
