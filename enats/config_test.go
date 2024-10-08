package enats

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_ToToml(t *testing.T) {
	conf := Config{
		Enabled:               true,
		Debug:                 false,
		Trace:                 true,
		Name:                  "test-service",
		ServiceAddr:           "localhost:4222",
		ClusterAddr:           "localhost:6222",
		ClusterName:           "test-cluster",
		GatewayAddr:           "localhost:7222",
		GatewayAdvertise:      "public.example.com:7222",
		Gateways:              []string{"nats://gateway1:7222", "nats://gateway2:7222"},
		Routes:                []string{"nats://route1:6222", "nats://route2:6222"},
		JetStream:             true,
		StoreDir:              "/tmp/nats-store",
		JetStreamReadyTimeout: 30,
	}

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "enabled = true")
	assert.Contains(t, tomlStr, "# debug = true")
	assert.Contains(t, tomlStr, "trace = true")
	assert.Contains(t, tomlStr, "name = \"test-service\"")
	assert.Contains(t, tomlStr, "service_addr = \"localhost:4222\"")
	assert.Contains(t, tomlStr, "cluster_addr = \"localhost:6222\"")
	assert.Contains(t, tomlStr, "cluster_name = \"test-cluster\"")
	assert.Contains(t, tomlStr, "gateway_addr = \"localhost:7222\"")
	assert.Contains(t, tomlStr, "gateway_advertise = \"public.example.com:7222\"")
	assert.Contains(t, tomlStr, "gateways = [\"nats://gateway1:7222\", \"nats://gateway2:7222\"]")
	assert.Contains(t, tomlStr, "routes = [\"nats://route1:6222\", \"nats://route2:6222\"]")
	assert.Contains(t, tomlStr, "jetstream = true")
	assert.Contains(t, tomlStr, "jetstream_store_dir = \"/tmp/nats-store\"")
	assert.Contains(t, tomlStr, "jetstream_ready_timeout = 30")

	// Round-trip test
	var conf2 Config
	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
