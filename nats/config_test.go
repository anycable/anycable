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
	conf.Channel = "test_channel"
	conf.DontRandomizeServers = true
	conf.MaxReconnectAttempts = 10
	conf.InternalChannel = "test_internal_channel"

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "servers = \"nats://localhost:4222\"")
	assert.Contains(t, tomlStr, "channel = \"test_channel\"")
	assert.Contains(t, tomlStr, "dont_randomize_servers = true")
	assert.Contains(t, tomlStr, "max_reconnect_attempts = 10")
	assert.Contains(t, tomlStr, "internal_channel = \"test_internal_channel\"")

	// Round-trip test
	conf2 := NewNATSConfig()

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
