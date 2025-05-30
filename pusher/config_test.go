package pusher

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_ToToml(t *testing.T) {
	conf := NewConfig()
	conf.AppKey = "test-app"

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "app_key = \"test-app\"")

	// Round-trip test
	conf2 := Config{}

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
