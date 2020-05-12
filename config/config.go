package config

import (
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/anycable/anycable-go/rpc"
	"github.com/anycable/anycable-go/server"
)

// Config contains main application configuration
type Config struct {
	RPC             rpc.Config
	Redis           pubsub.RedisConfig
	Host            string
	Port            int
	Path            string
	HealthPath      string
	Headers         []string
	SSL             server.SSLConfig
	WS              server.WSConfig
	MaxMessageSize  int64
	DisconnectQueue node.DisconnectQueueConfig
	LogLevel        string
	LogFormat       string
	Metrics         metrics.Config
}

// New returns a new empty config
func New() Config {
	config := Config{}
	config.SSL = server.NewSSLConfig()
	config.WS = server.NewWSConfig()
	config.Metrics = metrics.NewConfig()
	config.RPC = rpc.NewConfig()
	config.Redis = pubsub.NewRedisConfig()
	config.DisconnectQueue = node.NewDisconnectQueueConfig()
	return config
}
