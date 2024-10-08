package node

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_ToToml(t *testing.T) {
	conf := NewConfig()
	conf.DisconnectMode = "always"
	conf.HubGopoolSize = 100
	conf.PingTimestampPrecision = "ns"

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "disconnect_mode = \"always\"")
	assert.Contains(t, tomlStr, "broadcast_gopool_size = 100")
	assert.Contains(t, tomlStr, "ping_timestamp_precision = \"ns\"")
	assert.Contains(t, tomlStr, "# pong_timeout = 6")

	// Round-trip test
	conf2 := NewConfig()

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
