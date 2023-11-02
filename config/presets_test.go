package config

import (
	"os"
	"testing"

	"github.com/anycable/anycable-go/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoPresets(t *testing.T) {
	config := NewConfig()

	err := config.LoadPresets()

	require.NoError(t, err)

	assert.Equal(t, "localhost", config.Host)
}

func TestFlyPresets_no_vms_discovered(t *testing.T) {
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

	err := config.LoadPresets()

	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, 8989, config.Port)
	assert.Equal(t, 8989, config.HTTPBroadcast.Port)
	// No cluster info, no broker — sorry
	assert.Equal(t, "", config.BrokerAdapter)
	assert.Equal(t, "nats://0.0.0.0:4222", config.EmbeddedNats.ServiceAddr)
	assert.Equal(t, "nats://0.0.0.0:5222", config.EmbeddedNats.ClusterAddr)
	assert.Equal(t, "any-test-mag-cluster", config.EmbeddedNats.ClusterName)
	assert.Equal(t, []string{"nats://mag.any-test.internal:5222"}, config.EmbeddedNats.Routes)
	assert.Equal(t, "dns:///mag.anycable-web.internal:50051", config.RPC.Host)
}

func TestFlyPresets_when_single_vm_discovered(t *testing.T) {
	teardownDNS := mocks.MockDNSServer("vms.any-test.internal.", []string{"1234 mag"})
	defer teardownDNS()

	cleanupEnv := setEnv(
		"FLY_APP_NAME", "any-test",
		"FLY_REGION", "mag",
		"FLY_ALLOC_ID", "1234",
	)
	defer cleanupEnv()

	config := NewConfig()

	config.Port = 8989

	require.Equal(t, []string{"fly"}, config.Presets())

	err := config.LoadPresets()

	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, 8989, config.Port)
	assert.Equal(t, 8989, config.HTTPBroadcast.Port)
	// In-memory broker is good enough for single node, no pub/sub needed
	assert.Equal(t, false, config.EmbedNats)
	assert.Equal(t, "", config.PubSubAdapter)
	assert.Equal(t, "http", config.BroadcastAdapter)
	assert.Equal(t, "memory", config.BrokerAdapter)
	assert.Equal(t, "nats://0.0.0.0:4222", config.EmbeddedNats.ServiceAddr)
}

func TestFlyPresets_when_two_vms_discovered(t *testing.T) {
	teardownDNS := mocks.MockDNSServer("vms.any-test.internal.", []string{"1234 mag", "4567 mag"})
	defer teardownDNS()

	cleanupEnv := setEnv(
		"FLY_APP_NAME", "any-test",
		"FLY_REGION", "mag",
		"FLY_ALLOC_ID", "1234",
	)
	defer cleanupEnv()

	config := NewConfig()

	config.Port = 8989

	require.Equal(t, []string{"fly"}, config.Presets())

	err := config.LoadPresets()

	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, 8989, config.Port)
	assert.Equal(t, 8989, config.HTTPBroadcast.Port)
	assert.Equal(t, "http,nats", config.BroadcastAdapter)
	assert.Equal(t, true, config.EmbedNats)
	assert.Equal(t, "nats", config.PubSubAdapter)
	// We do not enable broker by default, since it requires at least 3 nodes or exactly 1
	assert.Equal(t, "", config.BrokerAdapter)
	assert.Equal(t, "nats://0.0.0.0:4222", config.EmbeddedNats.ServiceAddr)
	assert.Equal(t, "nats://0.0.0.0:5222", config.EmbeddedNats.ClusterAddr)
	assert.Equal(t, "any-test-mag-cluster", config.EmbeddedNats.ClusterName)
	assert.Equal(t, []string{"nats://mag.any-test.internal:5222"}, config.EmbeddedNats.Routes)
}

func TestFlyPresets_when_three_vms_discovered(t *testing.T) {
	teardownDNS := mocks.MockDNSServer("vms.any-test.internal.", []string{"1234 mag", "4567 mag", "8901 mag"})
	defer teardownDNS()

	cleanupEnv := setEnv(
		"FLY_APP_NAME", "any-test",
		"FLY_REGION", "mag",
		"FLY_ALLOC_ID", "1234",
	)
	defer cleanupEnv()

	config := NewConfig()

	config.Port = 8989

	require.Equal(t, []string{"fly"}, config.Presets())

	err := config.LoadPresets()

	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, 8989, config.Port)
	assert.Equal(t, 8989, config.HTTPBroadcast.Port)
	assert.Equal(t, "http,nats", config.BroadcastAdapter)
	assert.Equal(t, true, config.EmbedNats)
	assert.Equal(t, "nats", config.PubSubAdapter)
	assert.Equal(t, "nats", config.BrokerAdapter)
	assert.Equal(t, "nats://0.0.0.0:4222", config.EmbeddedNats.ServiceAddr)
	assert.Equal(t, "nats://0.0.0.0:5222", config.EmbeddedNats.ClusterAddr)
	assert.Equal(t, "any-test-mag-cluster", config.EmbeddedNats.ClusterName)
	assert.Equal(t, []string{"nats://mag.any-test.internal:5222"}, config.EmbeddedNats.Routes)
}

func TestFlyPresets_when_three_vms_from_different_regions(t *testing.T) {
	teardownDNS := mocks.MockDNSServer("vms.any-test.internal.", []string{"1234 mag", "4567 mag", "8901 sea"})
	defer teardownDNS()

	cleanupEnv := setEnv(
		"FLY_APP_NAME", "any-test",
		"FLY_REGION", "mag",
		"FLY_ALLOC_ID", "1234",
	)
	defer cleanupEnv()

	config := NewConfig()

	config.Port = 8989

	require.Equal(t, []string{"fly"}, config.Presets())

	err := config.LoadPresets()

	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, 8989, config.Port)
	assert.Equal(t, 8989, config.HTTPBroadcast.Port)
	assert.Equal(t, true, config.EmbedNats)
	assert.Equal(t, "http,nats", config.BroadcastAdapter)
	assert.Equal(t, "nats", config.PubSubAdapter)
	// Currently, we do not enable broker for multi-region setup; we need to figure this out later
	assert.Equal(t, "", config.BrokerAdapter)
	assert.Equal(t, "nats://0.0.0.0:4222", config.EmbeddedNats.ServiceAddr)
	assert.Equal(t, "nats://0.0.0.0:5222", config.EmbeddedNats.ClusterAddr)
	assert.Equal(t, "any-test-mag-cluster", config.EmbeddedNats.ClusterName)
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

	err := config.LoadPresets()

	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, false, config.EmbedNats)
	assert.Equal(t, "redis", config.BroadcastAdapter)
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

	err := config.LoadPresets()

	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, 4321, config.HTTPBroadcast.Port)
}

func TestBroker(t *testing.T) {
	config := NewConfig()
	config.UserPresets = []string{"broker"}
	config.BroadcastAdapter = "http"

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

	assert.Equal(t, "nats", config.BrokerAdapter)
	assert.Equal(t, "http,nats", config.BroadcastAdapter)
	assert.Equal(t, "nats", config.PubSubAdapter)
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

	err := config.LoadPresets()

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
