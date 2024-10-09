package ws

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_ToToml(t *testing.T) {
	conf := NewConfig()
	conf.Paths = []string{"/ws", "/socket"}
	conf.ReadBufferSize = 2048
	conf.WriteBufferSize = 2048
	conf.MaxMessageSize = 131072
	conf.EnableCompression = true

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "paths = [\"/ws\", \"/socket\"]")
	assert.Contains(t, tomlStr, "read_buffer_size = 2048")
	assert.Contains(t, tomlStr, "write_buffer_size = 2048")
	assert.Contains(t, tomlStr, "max_message_size = 131072")
	assert.Contains(t, tomlStr, "enable_compression = true")

	// Round-trip test
	conf2 := Config{}

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
