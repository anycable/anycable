package streams

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_ToToml(t *testing.T) {
	conf := NewConfig()
	conf.Secret = "test-secret"
	conf.Public = true
	conf.Whisper = false
	conf.PubSubChannel = "test-channel"
	conf.Turbo = true
	conf.TurboSecret = "turbo-secret"
	conf.CableReady = false
	conf.CableReadySecret = "cable-ready-secret"

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "secret = \"test-secret\"")
	assert.Contains(t, tomlStr, "public = true")
	assert.Contains(t, tomlStr, "# whisper = true")
	assert.Contains(t, tomlStr, "pubsub_channel = \"test-channel\"")
	assert.Contains(t, tomlStr, "turbo = true")
	assert.Contains(t, tomlStr, "turbo_secret = \"turbo-secret\"")
	assert.Contains(t, tomlStr, "# cable_ready = true")
	assert.Contains(t, tomlStr, "cable_ready_secret = \"cable-ready-secret\"")

	// Round-trip test
	conf2 := Config{}

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
