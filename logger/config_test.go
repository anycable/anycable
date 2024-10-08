package logger

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig__DecodeToml(t *testing.T) {
	tomlString := `
  level = "warn"
  format = "json"
  debug = true
 `

	conf := NewConfig()
	_, err := toml.Decode(tomlString, &conf)
	require.NoError(t, err)

	assert.Equal(t, "warn", conf.LogLevel)
	assert.Equal(t, "json", conf.LogFormat)
	assert.True(t, conf.Debug)
}

func TestConfig__ToToml(t *testing.T) {
	conf := NewConfig()
	conf.LogLevel = "warn"
	conf.LogFormat = "json"
	conf.Debug = false

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "level = \"warn\"")
	assert.Contains(t, tomlStr, "format = \"json\"")
	assert.Contains(t, tomlStr, "# debug = true")

	// Round-trip test
	conf2 := Config{}

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
