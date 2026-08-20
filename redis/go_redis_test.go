package redis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToGoRedisUniversalOptions(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://user:pass@localhost:6379/5"

	options, err := config.ToGoRedisUniversalOptions()
	require.NoError(t, err)

	assert.Equal(t, []string{"localhost:6379"}, options.Addrs)
	assert.Equal(t, "user", options.Username)
	assert.Equal(t, "pass", options.Password)
	assert.Equal(t, 5, options.DB)
	assert.Equal(t, config.MaxReconnectAttempts, options.MaxRetries)
}

func TestToGoRedisUniversalOptionsForCluster(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://localhost:6379,redis://localhost:6380"

	options, err := config.ToGoRedisUniversalOptions()
	require.NoError(t, err)

	assert.Equal(t, []string{"localhost:6379", "localhost:6380"}, options.Addrs)
	assert.Empty(t, options.MasterName)
}

func TestToGoRedisUniversalOptionsForSentinel(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://:data-pass@mymaster/3"
	config.Sentinels = ":sentinel-pass@localhost:26379,localhost:26380"

	options, err := config.ToGoRedisUniversalOptions()
	require.NoError(t, err)

	assert.Equal(t, []string{"localhost:26379", "localhost:26380"}, options.Addrs)
	assert.Equal(t, "mymaster", options.MasterName)
	assert.Equal(t, "data-pass", options.Password)
	assert.Equal(t, "sentinel-pass", options.SentinelPassword)
	assert.Equal(t, 3, options.DB)
}
