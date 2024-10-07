package config

import (
	"github.com/anycable/anycable-go/broadcast"
	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/enats"
	"github.com/anycable/anycable-go/identity"
	"github.com/anycable/anycable-go/logger"
	"github.com/anycable/anycable-go/metrics"
	nconfig "github.com/anycable/anycable-go/nats"
	"github.com/anycable/anycable-go/node"
	rconfig "github.com/anycable/anycable-go/redis"
	"github.com/anycable/anycable-go/rpc"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/sse"
	"github.com/anycable/anycable-go/streams"
	"github.com/anycable/anycable-go/ws"

	nanoid "github.com/matoous/go-nanoid"
)

// Config contains main application configuration
type Config struct {
	ID                   string
	Secret               string
	BroadcastKey         string
	SkipAuth             bool
	PublicMode           bool
	BroadcastAdapters    []string
	PubSubAdapter        string
	UserPresets          []string
	Log                  logger.Config
	Server               server.Config
	App                  node.Config
	WS                   ws.Config
	RPC                  rpc.Config
	Broker               broker.Config
	Redis                rconfig.RedisConfig
	HTTPBroadcast        broadcast.HTTPConfig
	NATS                 nconfig.NATSConfig
	DisconnectorDisabled bool
	DisconnectQueue      node.DisconnectQueueConfig
	Metrics              metrics.Config
	JWT                  identity.JWTConfig
	EmbeddedNats         enats.Config
	SSE                  sse.Config
	Streams              streams.Config
}

// NewConfig returns a new empty config
func NewConfig() Config {
	id, _ := nanoid.Nanoid(6)

	config := Config{
		ID:     id,
		Server: server.NewConfig(),
		// TODO(v2.0): Make HTTP default
		BroadcastAdapters: []string{"http", "redis"},
		Broker:            broker.NewConfig(),
		Log:               logger.NewConfig(),
		App:               node.NewConfig(),
		WS:                ws.NewConfig(),
		Metrics:           metrics.NewConfig(),
		RPC:               rpc.NewConfig(),
		Redis:             rconfig.NewRedisConfig(),
		HTTPBroadcast:     broadcast.NewHTTPConfig(),
		NATS:              nconfig.NewNATSConfig(),
		DisconnectQueue:   node.NewDisconnectQueueConfig(),
		JWT:               identity.NewJWTConfig(""),
		EmbeddedNats:      enats.NewConfig(),
		SSE:               sse.NewConfig(),
		Streams:           streams.NewConfig(),
	}

	return config
}

func (c Config) IsPublic() bool {
	return c.SkipAuth && c.Streams.Public
}
