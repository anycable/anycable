package config

import (
	"github.com/anycable/anycable-go/broadcast"
	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/enats"
	"github.com/anycable/anycable-go/identity"
	"github.com/anycable/anycable-go/metrics"
	nconfig "github.com/anycable/anycable-go/nats"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/rails"
	rconfig "github.com/anycable/anycable-go/redis"
	"github.com/anycable/anycable-go/rpc"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/ws"
)

// Config contains main application configuration
type Config struct {
	App                  node.Config
	RPC                  rpc.Config
	BrokerAdapter        string
	Broker               broker.Config
	Redis                rconfig.RedisConfig
	HTTPBroadcast        broadcast.HTTPConfig
	NATS                 nconfig.NATSConfig
	Host                 string
	Port                 int
	MaxConn              int
	BroadcastAdapter     string
	PubSubAdapter        string
	Path                 []string
	HealthPath           string
	Headers              []string
	Cookies              []string
	SSL                  server.SSLConfig
	WS                   ws.Config
	MaxMessageSize       int64
	DisconnectorDisabled bool
	DisconnectQueue      node.DisconnectQueueConfig
	LogLevel             string
	LogFormat            string
	Debug                bool
	Metrics              metrics.Config
	JWT                  identity.JWTConfig
	Rails                rails.Config
	EmbedNats            bool
	EmbeddedNats         enats.Config
	UserPresets          []string
}

// NewConfig returns a new empty config
func NewConfig() Config {
	config := Config{
		Host:             "localhost",
		Port:             8080,
		Path:             []string{"/cable"},
		HealthPath:       "/health",
		BroadcastAdapter: "redis",
		Broker:           broker.NewConfig(),
		Headers:          []string{"cookie"},
		LogLevel:         "info",
		LogFormat:        "text",
		App:              node.NewConfig(),
		SSL:              server.NewSSLConfig(),
		WS:               ws.NewConfig(),
		Metrics:          metrics.NewConfig(),
		RPC:              rpc.NewConfig(),
		Redis:            rconfig.NewRedisConfig(),
		HTTPBroadcast:    broadcast.NewHTTPConfig(),
		NATS:             nconfig.NewNATSConfig(),
		DisconnectQueue:  node.NewDisconnectQueueConfig(),
		JWT:              identity.NewJWTConfig(""),
		Rails:            rails.NewConfig(),
		EmbeddedNats:     enats.NewConfig(),
	}

	return config
}
