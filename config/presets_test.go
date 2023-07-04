package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoPresets(t *testing.T) {
	config := NewConfig()

	err := config.LoadPresets()

	require.NoError(t, err)

	assert.Equal(t, "localhost", config.Host)
}

func TestFlyPresets(t *testing.T) {
	setEnv(
		"FLY_APP_NAME", "any-test",
		"FLY_REGION", "mag",
		"FLY_ALLOC_ID", "1234",
		"ANYCABLE_FLY_RPC_APP_NAME", "anycable-web",
	)

	defer unsetEnv("FLY_APP_NAME", "FLY_REGION", "FLY_ALLOC_ID", "ANYCABLE_FLY_RPC_APP_NAME")

	config := NewConfig()

	require.Equal(t, []string{"fly"}, config.Presets())

	err := config.LoadPresets()

	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, "nats://0.0.0.0:4222", config.EmbeddedNats.ServiceAddr)
	assert.Equal(t, "nats://0.0.0.0:5222", config.EmbeddedNats.ClusterAddr)
	assert.Equal(t, "any-test-mag-cluster", config.EmbeddedNats.ClusterName)
	assert.Equal(t, []string{"nats://mag.any-test.internal:5222"}, config.EmbeddedNats.Routes)
	assert.Equal(t, "dns:///mag.anycable-web.internal:50051", config.RPC.Host)
}

func TestHerokuPresets(t *testing.T) {
	setEnv(
		"HEROKU_DYNO_ID", "web.42",
		"HEROKU_APP_ID", "herr-cable",
		"PORT", "4321",
	)

	defer unsetEnv("HEROKU_DYNO_ID", "HEROKU_APP_ID", "PORT")

	config := NewConfig()

	require.Equal(t, []string{"heroku"}, config.Presets())

	err := config.LoadPresets()

	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, 4321, config.HTTPBroadcast.Port)
}

func TestBroker(t *testing.T) {
	config := NewConfig()
	config.UserPresets = []string{"broker"}

	require.Equal(t, []string{"broker"}, config.Presets())

	err := config.LoadPresets()

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

	err := config.LoadPresets()

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

	err := config.LoadPresets()

	require.NoError(t, err)

	assert.Equal(t, "memory", config.BrokerAdapter)
	assert.Equal(t, "http,nats", config.BroadcastAdapter)
	assert.Equal(t, "nats", config.PubSubAdapter)
}

func TestOverrideSomePresetSettings(t *testing.T) {
	setEnv(
		"FLY_APP_NAME", "any-test",
		"FLY_REGION", "mag",
		"FLY_ALLOC_ID", "1234",
	)

	defer unsetEnv("FLY_APP_NAME", "FLY_REGION", "FLY_ALLOC_ID")

	config := NewConfig()

	require.Equal(t, []string{"fly"}, config.Presets())

	config.EmbeddedNats.ServiceAddr = "nats://0.0.0.0:1234"

	err := config.LoadPresets()

	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, "nats://0.0.0.0:1234", config.EmbeddedNats.ServiceAddr)
}

func TestExplicitOverImplicitPresets(t *testing.T) {
	setEnv(
		"FLY_APP_NAME", "any-test",
		"FLY_REGION", "mag",
		"FLY_ALLOC_ID", "1234",
	)

	defer unsetEnv("FLY_APP_NAME", "FLY_REGION", "FLY_ALLOC_ID")

	config := NewConfig()
	config.UserPresets = []string{}

	assert.Equal(t, []string{}, config.Presets())
}

func setEnv(pairs ...string) {
	for i := 0; i < len(pairs); i += 2 {
		os.Setenv(pairs[i], pairs[i+1])
	}
}

func unsetEnv(keys ...string) {
	for _, key := range keys {
		os.Unsetenv(key)
	}
}
