package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
