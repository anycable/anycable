package redis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasic(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://localhost:6379"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.False(t, config.IsCluster())
	assert.False(t, config.IsSentinel())

	assert.Equal(t, "localhost:6379", config.Hostname())
	assert.Equal(t, []string{"localhost:6379"}, config.Hostnames())

	assert.Equal(t, []string{"localhost:6379"}, options.InitAddress)
	assert.Equal(t, 0, options.SelectDB)
	assert.Equal(t, 30*time.Second, options.Dialer.KeepAlive)
	assert.False(t, options.ShuffleInit)
	assert.Nil(t, options.TLSConfig)
}

func TestCustomDatabase(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://localhost:6379/1"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.Equal(t, 1, options.SelectDB)
}

func TestCustomOptions(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://localhost:6379/1?dial_timeout=30s"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.Equal(t, 30*time.Second, options.Dialer.Timeout)
}

func TestTLS(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "rediss://localhost:6379/1"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.True(t, options.TLSConfig.InsecureSkipVerify)

	config.TLSVerify = true
	options, err = config.ToRueidisOptions()
	require.NoError(t, err)

	assert.False(t, options.TLSConfig.InsecureSkipVerify)
}

func TestAuth(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://user:pass@localhost:6379/1"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.Equal(t, "user", options.Username)
	assert.Equal(t, "pass", options.Password)
}

func TestCluster(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://localhost:6379/1,redis://localhost:6389/1"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.True(t, config.IsCluster())
	assert.False(t, config.IsSentinel())

	assert.Equal(t, "localhost:6379", config.Hostname())
	assert.Equal(t, []string{"localhost:6379", "localhost:6389"}, config.Hostnames())

	assert.Equal(t, []string{"localhost:6379", "localhost:6389"}, options.InitAddress)
	assert.True(t, options.ShuffleInit)
}

func TestClusterShortSyntax(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://localhost:6379/1,localhost:6389/1"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.True(t, config.IsCluster())
	assert.False(t, config.IsSentinel())

	assert.Equal(t, "localhost:6379", config.Hostname())
	assert.Equal(t, []string{"localhost:6379", "localhost:6389"}, config.Hostnames())

	assert.Equal(t, []string{"localhost:6379", "localhost:6389"}, options.InitAddress)
	assert.True(t, options.ShuffleInit)
}

func TestSentinel(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://master-name"
	config.Sentinels = "user:pass@localhost:1234,localhost:1235"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.False(t, config.IsCluster())
	assert.True(t, config.IsSentinel())

	assert.Equal(t, "master-name", config.Hostname())
	assert.Equal(t, []string{"localhost:1234", "localhost:1235"}, config.Hostnames())

	assert.Equal(t, []string{"localhost:1234", "localhost:1235"}, options.InitAddress)
	assert.Equal(t, "user", options.Username)
	assert.Equal(t, "pass", options.Password)
}

func TestSentinelImplicitFormat(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "redis://user:pass@localhost:1234?master_set=master-name"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.False(t, config.IsCluster())
	assert.True(t, config.IsSentinel())

	assert.Equal(t, "master-name", config.Hostname())
	assert.Equal(t, []string{"localhost:1234"}, config.Hostnames())

	assert.Equal(t, []string{"localhost:1234"}, options.InitAddress)
	assert.Equal(t, "user", options.Username)
	assert.Equal(t, "pass", options.Password)
}

func TestDefaultScheme(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "localhost"
	options, err := config.ToRueidisOptions()
	require.NoError(t, err)

	assert.Equal(t, "localhost:6379", config.Hostname())
	assert.Equal(t, []string{"localhost:6379"}, config.Hostnames())

	assert.Equal(t, []string{"localhost:6379"}, options.InitAddress)
}

func TestInvalidURL(t *testing.T) {
	config := NewRedisConfig()
	config.URL = "invalid://"
	_, err := config.ToRueidisOptions()
	require.Error(t, err)
}
