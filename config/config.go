package config

import (
	"github.com/anycable/anycable-go/admin"
	"github.com/anycable/anycable-go/broadcast"
	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/enats"
	"github.com/anycable/anycable-go/graphql"
	"github.com/anycable/anycable-go/identity"
	"github.com/anycable/anycable-go/lp"
	"github.com/anycable/anycable-go/metrics"
	nconfig "github.com/anycable/anycable-go/nats"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/ocpp"
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
	EmbedNats            bool
	EmbeddedNats         enats.Config
	SSE                  sse.Config
	Streams              streams.Config
	UserPresets          []string
	GraphQL              graphql.Config
	LegacyGraphQL        graphql.Config
	OCPP                 ocpp.Config
	LongPolling          lp.Config
	Admin                admin.Config
}

// NewConfig returns a new empty config
func NewConfig() Config {
	id, _ := nanoid.Nanoid(6)

	config := Config{
		ID:               id,
		Host:             "localhost",
		Port:             8080,
		Path:             []string{"/cable"},
		HealthPath:       "/health",
		BroadcastAdapter: "http,redis",
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
		EmbeddedNats:     enats.NewConfig(),
		SSE:              sse.NewConfig(),
		Streams:          streams.NewConfig(),
		GraphQL:          graphql.NewConfig(),
		LegacyGraphQL:    graphql.NewConfig(),
		OCPP:             ocpp.NewConfig(),
		LongPolling:      lp.NewConfig(),
		Admin:            admin.NewConfig(),
	}

	return config
}

func (c Config) IsPublic() bool {
	return c.SkipAuth && c.Streams.Public
}
