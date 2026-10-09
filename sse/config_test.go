package sse

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_ToToml(t *testing.T) {
	conf := NewConfig()
	conf.Path = "/events"

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "path = \"/events\"")
	assert.Contains(t, tomlStr, "# enabled = true")

	// Round-trip test
	conf2 := Config{}

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}

func TestConfig_ToToml_CommentPings(t *testing.T) {
	conf := NewConfig()

	assert.Contains(t, conf.ToToml(), "# comment_pings = true")

	conf.CommentPings = true
	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "\ncomment_pings = true")

	conf2 := Config{}

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
