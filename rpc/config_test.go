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

	c.Host = "localhost:50051/anycable"
	assert.Equal(t, "grpc", c.Impl())
}
