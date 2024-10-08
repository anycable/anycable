package broker

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig__DecodeToml(t *testing.T) {
	tomlString := `
  adapter = "nats"
  history_ttl = 100
  history_limit = 1000
  sessions_ttl = 600
 `

	conf := NewConfig()
	_, err := toml.Decode(tomlString, &conf)
	require.NoError(t, err)

	assert.Equal(t, "nats", conf.Adapter)
	assert.Equal(t, int64(100), conf.HistoryTTL)
	assert.Equal(t, 1000, conf.HistoryLimit)
	assert.Equal(t, int64(600), conf.SessionsTTL)
}

func TestConfig__ToToml(t *testing.T) {
	conf := NewConfig()
	conf.Adapter = "nats"
	conf.HistoryTTL = 100
	conf.HistoryLimit = 1000
	conf.SessionsTTL = 600

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "adapter = \"nats\"")
	assert.Contains(t, tomlStr, "history_ttl = 100")
	assert.Contains(t, tomlStr, "history_limit = 1000")
	assert.Contains(t, tomlStr, "sessions_ttl = 600")

	// Round-trip test
	conf2 := Config{}

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
