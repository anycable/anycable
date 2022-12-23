package redis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasic(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://localhost:6379/1"
	err := config.Parse()
	require.NoError(t, err)

	assert.Equal(t, "localhost:6379", config.Hostname())
	assert.False(t, config.HasTLS())
	assert.False(t, config.HasAuth())
	assert.False(t, config.IsCluster())
	assert.False(t, config.IsSentinel())
}

func TestTLS(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "rediss://localhost:6379/1"
	err := config.Parse()
	require.NoError(t, err)

	assert.Equal(t, "localhost:6379", config.Hostname())
	assert.True(t, config.HasTLS())

	tls := config.ToTLSConfig()
	assert.Equal(t, true, tls.InsecureSkipVerify)

	config.TLSVerify = true
	err = config.Parse()
	require.NoError(t, err)

	tls = config.ToTLSConfig()
	assert.Equal(t, false, tls.InsecureSkipVerify)
}

func TestAuth(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://user:pass@localhost:6379/1"
	err := config.Parse()
	require.NoError(t, err)

	assert.Equal(t, "localhost:6379", config.Hostname())
	assert.True(t, config.HasAuth())
	assert.Equal(t, "user", config.Username())
	assert.Equal(t, "pass", config.Password())
}

func TestCluster(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://localhost:6379/1,redis://localhost:6389/1"
	err := config.Parse()
	require.NoError(t, err)

	assert.Equal(t, []string{"localhost:6379", "localhost:6389"}, config.Hostnames())
	assert.True(t, config.IsCluster())
}

func TestSentinel(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://master-name"
	config.Sentinels = "user:pass@localhost:1234,localhost:1235"
	err := config.Parse()
	require.NoError(t, err)

	require.True(t, config.IsSentinel())
	assert.Equal(t, "master-name", config.Hostname())
	assert.False(t, config.HasTLS())
	assert.True(t, config.HasAuth())
	assert.Equal(t, "user", config.Username())
	assert.Equal(t, "pass", config.Password())
	assert.False(t, config.IsCluster())
	assert.Equal(t, []string{"localhost:1234", "localhost:1235"}, config.SentinelHostnames())
}
