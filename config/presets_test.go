package config

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoPresets(t *testing.T) {
	config := NewConfig()

	err := config.LoadPresets(slog.Default())

	require.NoError(t, err)

	assert.Equal(t, "localhost", config.Host)
}

func TestFlyPresets(t *testing.T) {
	cleanupEnv := setEnv(
		"FLY_APP_NAME", "any-test",
		"FLY_REGION", "mag",
		"FLY_ALLOC_ID", "1234",
		"ANYCABLE_FLY_RPC_APP_NAME", "anycable-web",
	)
	defer cleanupEnv()

	config := NewConfig()

	config.Port = 8989

	require.Equal(t, []string{"fly"}, config.Presets())

	err := config.LoadPresets(slog.Default())

	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, 8989, config.Port)
	assert.Equal(t, 8989, config.HTTPBroadcast.Port)
	assert.Equal(t, true, config.EmbedNats)
	assert.Equal(t, "nats", config.PubSubAdapter)
	assert.Equal(t, "", config.BrokerAdapter)
	assert.Contains(t, config.EmbeddedNats.Name, "fly-mag-1234-")
	assert.Equal(t, "nats://0.0.0.0:4222", config.EmbeddedNats.ServiceAddr)
	assert.Equal(t, "nats://0.0.0.0:5222", config.EmbeddedNats.ClusterAddr)
	assert.Equal(t, "any-test-mag-cluster", config.EmbeddedNats.ClusterName)
	assert.Equal(t, []string{"nats://mag.any-test.internal:5222"}, config.EmbeddedNats.Routes)
	assert.Equal(t, "dns:///mag.anycable-web.internal:50051", config.RPC.Host)
}

func TestFlyPresets_When_RedisConfigured(t *testing.T) {
	cleanupEnv := setEnv(
		"FLY_APP_NAME", "any-test",
		"FLY_REGION", "mag",
		"FLY_ALLOC_ID", "1234",
	)
	defer cleanupEnv()

	config := NewConfig()
	config.Redis.URL = "redis://some.cloud.redis:6379/1"

	require.Equal(t, []string{"fly"}, config.Presets())

	err := config.LoadPresets(slog.Default())

	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, false, config.EmbedNats)
	assert.Equal(t, "http,redis", config.BroadcastAdapter)
	assert.Equal(t, "redis", config.PubSubAdapter)
	assert.Equal(t, "nats://0.0.0.0:4222", config.EmbeddedNats.ServiceAddr)
	assert.Equal(t, "nats://0.0.0.0:5222", config.EmbeddedNats.ClusterAddr)
	assert.Equal(t, "any-test-mag-cluster", config.EmbeddedNats.ClusterName)
	assert.Equal(t, []string{"nats://mag.any-test.internal:5222"}, config.EmbeddedNats.Routes)
	// It stays default
	assert.Equal(t, "localhost:50051", config.RPC.Host)
}

func TestHerokuPresets(t *testing.T) {
	cleanupEnv := setEnv(
		"HEROKU_DYNO_ID", "web.42",
		"HEROKU_APP_ID", "herr-cable",
		"PORT", "4321",
	)
	defer cleanupEnv()

	config := NewConfig()

	require.Equal(t, []string{"heroku"}, config.Presets())

	err := config.LoadPresets(slog.Default())

	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, 4321, config.HTTPBroadcast.Port)
}

func TestBroker(t *testing.T) {
	config := NewConfig()
	config.UserPresets = []string{"broker"}
	config.BroadcastAdapter = "http"

	require.Equal(t, []string{"broker"}, config.Presets())

	err := config.LoadPresets(slog.Default())

	require.NoError(t, err)

	assert.Equal(t, "memory", config.BrokerAdapter)
	assert.Equal(t, "http", config.BroadcastAdapter)
	assert.Equal(t, "", config.PubSubAdapter)
}

func TestBrokerWhenRedisConfigured(t *testing.T) {
	config := NewConfig()
	config.UserPresets = []string{"broker"}
	config.Redis.URL = "redis://localhost:6379/1"

	require.Equal(t, []string{"broker"}, config.Presets())

	err := config.LoadPresets(slog.Default())

	require.NoError(t, err)

	assert.Equal(t, "memory", config.BrokerAdapter)
	assert.Equal(t, "http,redisx,redis", config.BroadcastAdapter)
	assert.Equal(t, "redis", config.PubSubAdapter)
}

func TestBrokerWhenENATSConfigured(t *testing.T) {
	config := NewConfig()
	config.UserPresets = []string{"broker"}
	config.EmbedNats = true

	require.Equal(t, []string{"broker"}, config.Presets())

	err := config.LoadPresets(slog.Default())

	require.NoError(t, err)

	assert.Equal(t, "nats", config.BrokerAdapter)
	assert.Equal(t, "http,nats", config.BroadcastAdapter)
	assert.Equal(t, "nats", config.PubSubAdapter)
}

func TestFlyWithBrokerPresets(t *testing.T) {
	cleanupEnv := setEnv(
		"FLY_APP_NAME", "any-test",
		"FLY_REGION", "mag",
		"FLY_ALLOC_ID", "1234",
	)
	defer cleanupEnv()

	config := NewConfig()
	config.UserPresets = []string{"fly", "broker"}
	config.Port = 8989

	require.Equal(t, []string{"fly", "broker"}, config.Presets())

	err := config.LoadPresets(slog.Default())

	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, 8989, config.Port)
	assert.Equal(t, 8989, config.HTTPBroadcast.Port)
	assert.Equal(t, true, config.EmbedNats)
	assert.Equal(t, "nats", config.PubSubAdapter)
	assert.Equal(t, "nats", config.BrokerAdapter)
	assert.Equal(t, "http,nats", config.BroadcastAdapter)
	assert.Equal(t, "nats://0.0.0.0:4222", config.EmbeddedNats.ServiceAddr)
	assert.Equal(t, "nats://0.0.0.0:5222", config.EmbeddedNats.ClusterAddr)
	assert.Equal(t, "any-test-mag-cluster", config.EmbeddedNats.ClusterName)
	assert.Equal(t, []string{"nats://mag.any-test.internal:5222"}, config.EmbeddedNats.Routes)
	assert.Equal(t, "localhost:50051", config.RPC.Host)
}

func TestOverrideSomePresetSettings(t *testing.T) {
	cleanupEnv := setEnv(
		"FLY_APP_NAME", "any-test",
		"FLY_REGION", "mag",
		"FLY_ALLOC_ID", "1234",
	)
	defer cleanupEnv()

	config := NewConfig()

	require.Equal(t, []string{"fly"}, config.Presets())

	config.EmbeddedNats.ServiceAddr = "nats://0.0.0.0:1234"

	err := config.LoadPresets(slog.Default())

	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, "nats://0.0.0.0:1234", config.EmbeddedNats.ServiceAddr)
}

func TestExplicitOverImplicitPresets(t *testing.T) {
	cleanupEnv := setEnv(
		"FLY_APP_NAME", "any-test",
		"FLY_REGION", "mag",
		"FLY_ALLOC_ID", "1234",
	)
	defer cleanupEnv()

	config := NewConfig()
	config.UserPresets = []string{}

	assert.Equal(t, []string{}, config.Presets())
}

func setEnv(pairs ...string) func() {
	keys := []string{}

	for i := 0; i < len(pairs); i += 2 {
		keys = append(keys, pairs[i])
		os.Setenv(pairs[i], pairs[i+1])
	}

	return func() {
		unsetEnv(keys...)
	}
}

func unsetEnv(keys ...string) {
	for _, key := range keys {
		os.Unsetenv(key)
	}
}
