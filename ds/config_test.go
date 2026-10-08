package ds

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_ToToml(t *testing.T) {
	conf := NewConfig()
	conf.Path = "/streams/v1"

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "path = \"/streams/v1\"")
	assert.Contains(t, tomlStr, "# enabled = true")

	// Round-trip test
	conf2 := Config{}

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}

func TestConfig_ToToml_SSEPingInterval(t *testing.T) {
	conf := NewConfig()

	assert.Contains(t, conf.ToToml(), "# sse_ping_interval = 15")

	conf.SSEPingInterval = 30
	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "\nsse_ping_interval = 30")

	conf2 := Config{}

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
