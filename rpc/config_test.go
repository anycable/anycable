package rpc

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Impl(t *testing.T) {
	c := NewConfig()

	c.Implementation = "http"
	assert.Equal(t, "http", c.Impl())

	c.Implementation = "trpc"
	assert.Equal(t, "trpc", c.Impl())

	c.Implementation = ""
	assert.Equal(t, "grpc", c.Impl())

	c.Host = "http://localhost:8080/anycable"
	assert.Equal(t, "http", c.Impl())

	c.Host = "https://localhost:8080/anycable"
	assert.Equal(t, "http", c.Impl())

	c.Host = "grpc://localhost:50051/anycable"
	assert.Equal(t, "grpc", c.Impl())

	c.Host = "dns:///rpc:50051"
	assert.Equal(t, "grpc", c.Impl())

	c.Host = "localhost:50051/anycable"
	assert.Equal(t, "grpc", c.Impl())

	c.Host = "127.0.0.1:50051/anycable"
	assert.Equal(t, "grpc", c.Impl())

	c.Host = "invalid://:+"
	assert.Equal(t, "<invalid RPC host: invalid://:+>", c.Impl())
}

func TestConfig__ToToml(t *testing.T) {
	conf := NewConfig()
	conf.Host = "rpc.test"
	conf.Concurrency = 10
	conf.Implementation = "http"
	conf.ProxyHeaders = []string{"Cookie", "X-Api-Key"}
	conf.ProxyCookies = []string{"_session_id", "_csrf_token"}

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "implementation = \"http\"")
	assert.Contains(t, tomlStr, "host = \"rpc.test\"")
	assert.Contains(t, tomlStr, "concurrency = 10")
	assert.Contains(t, tomlStr, "proxy_headers = [\"Cookie\", \"X-Api-Key\"]")
	assert.Contains(t, tomlStr, "proxy_cookies = [\"_session_id\", \"_csrf_token\"]")

	// Round-trip test
	conf2 := Config{}

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
