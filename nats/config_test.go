package nats

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNATSConfig_ToToml(t *testing.T) {
	conf := NewNATSConfig()
	conf.Servers = "nats://localhost:4222"
	conf.DontRandomizeServers = true
	conf.MaxReconnectAttempts = 10

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "servers = \"nats://localhost:4222\"")
	assert.Contains(t, tomlStr, "dont_randomize_servers = true")
	assert.Contains(t, tomlStr, "max_reconnect_attempts = 10")

	// Round-trip test
	conf2 := NewNATSConfig()

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
