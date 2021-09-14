package config

import (
	"github.com/anycable/anycable-go/apollo"
	"github.com/anycable/anycable-go/identity"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/anycable/anycable-go/rails"
	"github.com/anycable/anycable-go/rpc"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/ws"
)

// Config contains main application configuration
type Config struct {
	App                  node.Config
	RPC                  rpc.Config
	Redis                pubsub.RedisConfig
	HTTPPubSub           pubsub.HTTPConfig
	Host                 string
	Port                 int
	MaxConn              int
	BroadcastAdapter     string
	Path                 string
	HealthPath           string
	Headers              []string
	SSL                  server.SSLConfig
	WS                   ws.Config
	MaxMessageSize       int64
	DisconnectorDisabled bool
	DisconnectQueue      node.DisconnectQueueConfig
	LogLevel             string
	LogFormat            string
	Metrics              metrics.Config
	Apollo               apollo.Config
	JWT                  identity.JWTConfig
	Rails                rails.Config
}

// New returns a new empty config
func New() Config {
	config := Config{}
	config.App = node.NewConfig()
	config.SSL = server.NewSSLConfig()
	config.WS = ws.NewConfig()
	config.Metrics = metrics.NewConfig()
	config.RPC = rpc.NewConfig()
	config.Redis = pubsub.NewRedisConfig()
	config.HTTPPubSub = pubsub.NewHTTPConfig()
	config.DisconnectQueue = node.NewDisconnectQueueConfig()
	config.Apollo = apollo.NewConfig()
	config.JWT = identity.NewJWTConfig("")
	config.Rails = rails.NewConfig()
	return config
}
