package broadcast

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLegacyRedisConfig__ToToml(t *testing.T) {
	conf := NewLegacyRedisConfig()
	conf.Channel = "_test_"

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "channel = \"_test_\"")

	// Round-trip test
	conf2 := NewLegacyRedisConfig()

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
