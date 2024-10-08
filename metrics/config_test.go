package metrics

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogEnabled(t *testing.T) {
	config := NewConfig()
	assert.False(t, config.LogEnabled())

	config.Log = true
	assert.True(t, config.LogEnabled())

	config.Log = false
	assert.False(t, config.LogEnabled())

	config = NewConfig()
	config.LogFormatter = "test"
	assert.True(t, config.LogEnabled())
}

func TestHTTPEnabled(t *testing.T) {
	config := NewConfig()
	assert.False(t, config.HTTPEnabled())

	config.HTTP = "/metrics"
	assert.True(t, config.HTTPEnabled())
}

func TestConfig_ToToml(t *testing.T) {
	conf := NewConfig()
	conf.Log = true
	conf.RotateInterval = 30
	conf.LogFilter = []string{"metric1", "metric2"}
	conf.Host = "example.com"
	conf.Port = 9090
	conf.Tags = map[string]string{"env": "prod", "region": "us-west"}

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "host = \"example.com\"")
	assert.Contains(t, tomlStr, "port = 9090")
	assert.Contains(t, tomlStr, "log = true")
	assert.Contains(t, tomlStr, "rotate_interval = 30")
	assert.Contains(t, tomlStr, "log_filter = [ \"metric1\", \"metric2\" ]")
	assert.Contains(t, tomlStr, "tags.env = \"prod\"")
	assert.Contains(t, tomlStr, "tags.region = \"us-west\"")

	// Round-trip test
	conf2 := NewConfig()

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
