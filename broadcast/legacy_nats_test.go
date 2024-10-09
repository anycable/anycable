package broadcast

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLegacyNATSConfig__ToToml(t *testing.T) {
	conf := NewLegacyNATSConfig()
	conf.Channel = "_test_"

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "channel = \"_test_\"")

	// Round-trip test
	conf2 := NewLegacyNATSConfig()

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
