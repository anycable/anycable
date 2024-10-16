package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/anycable/anycable-go/broadcast"
	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/enats"
	"github.com/anycable/anycable-go/identity"
	"github.com/anycable/anycable-go/logger"
	"github.com/anycable/anycable-go/metrics"
	nconfig "github.com/anycable/anycable-go/nats"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/pubsub"
	rconfig "github.com/anycable/anycable-go/redis"
	"github.com/anycable/anycable-go/rpc"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/sse"
	"github.com/anycable/anycable-go/streams"
	"github.com/anycable/anycable-go/ws"
	"github.com/joomcode/errorx"

	nanoid "github.com/matoous/go-nanoid"
)

// Config contains main application configuration
type Config struct {
	ID                   string `toml:"node_id"`
	UserProvidedID       bool
	Secret               string                      `toml:"secret"`
	BroadcastKey         string                      `toml:"broadcast_key"`
	SkipAuth             bool                        `toml:"noauth"`
	PublicMode           bool                        `toml:"public"`
	BroadcastAdapters    []string                    `toml:"broadcast_adapters"`
	PubSubAdapter        string                      `toml:"pubsub_adapter"`
	UserPresets          []string                    `toml:"presets"`
	Log                  logger.Config               `toml:"logging"`
	Server               server.Config               `toml:"server"`
	App                  node.Config                 `toml:"app"`
	WS                   ws.Config                   `toml:"ws"`
	RPC                  rpc.Config                  `toml:"rpc"`
	Broker               broker.Config               `toml:"broker"`
	Redis                rconfig.RedisConfig         `toml:"redis"`
	LegacyRedisBroadcast broadcast.LegacyRedisConfig `toml:"redis_pubsub_broadcast"`
	RedisBroadcast       broadcast.RedisConfig       `toml:"redis_stream_broadcast"`
	NATSBroadcast        broadcast.LegacyNATSConfig  `toml:"nats_broadcast"`
	HTTPBroadcast        broadcast.HTTPConfig        `toml:"http_broadcast"`
	RedisPubSub          pubsub.RedisConfig          `toml:"redis_pubsub"`
	NATSPubSub           pubsub.NATSConfig           `toml:"nats_pubsub"`
	NATS                 nconfig.NATSConfig          `toml:"nats"`
	DisconnectorDisabled bool
	DisconnectQueue      node.DisconnectQueueConfig `toml:"disconnector"`
	Metrics              metrics.Config             `toml:"metrics"`
	JWT                  identity.JWTConfig         `toml:"jwt"`
	EmbeddedNats         enats.Config               `toml:"embedded_nats"`
	SSE                  sse.Config                 `toml:"sse"`
	Streams              streams.Config             `toml:"streams"`

	ConfigFilePath string
}

// NewConfig returns a new empty config
func NewConfig() Config {
	id, _ := nanoid.Nanoid(6)

	config := Config{
		ID:     id,
		Server: server.NewConfig(),
		// TODO(v2.0): Make HTTP default
		BroadcastAdapters:    []string{"http", "redis"},
		Broker:               broker.NewConfig(),
		Log:                  logger.NewConfig(),
		App:                  node.NewConfig(),
		WS:                   ws.NewConfig(),
		Metrics:              metrics.NewConfig(),
		RPC:                  rpc.NewConfig(),
		Redis:                rconfig.NewRedisConfig(),
		RedisBroadcast:       broadcast.NewRedisConfig(),
		LegacyRedisBroadcast: broadcast.NewLegacyRedisConfig(),
		NATSBroadcast:        broadcast.NewLegacyNATSConfig(),
		HTTPBroadcast:        broadcast.NewHTTPConfig(),
		RedisPubSub:          pubsub.NewRedisConfig(),
		NATSPubSub:           pubsub.NewNATSConfig(),
		NATS:                 nconfig.NewNATSConfig(),
		DisconnectQueue:      node.NewDisconnectQueueConfig(),
		JWT:                  identity.NewJWTConfig(""),
		EmbeddedNats:         enats.NewConfig(),
		SSE:                  sse.NewConfig(),
		Streams:              streams.NewConfig(),
	}

	return config
}

func (c *Config) LoadFromFile() error {
	bytes, err := os.ReadFile(c.ConfigFilePath)

	if err != nil {
		return errorx.Decorate(err, "failed to read config file")
	}

	prevID := c.ID
	_, err = toml.Decode(string(bytes), &c)

	if err != nil {
		return errorx.Decorate(err, "failed to parse TOML configuration")
	}

	if c.ID != prevID {
		c.UserProvidedID = true
	}

	return nil
}

func (c Config) ToToml() string {
	var result strings.Builder

	result.WriteString("# AnyCable server configuration.\n# Read more at https://docs.anycable.io/anycable-go/configuration\n\n")

	result.WriteString("# General settings\n\n")

	result.WriteString("# Public mode disables connection authentication, pub/sub streams and broadcasts verification\n")
	if c.PublicMode {
		result.WriteString(fmt.Sprintf("public = %t\n\n", c.PublicMode))
	} else {
		result.WriteString("# public = false\n\n")
	}

	result.WriteString("# Disable connection authentication only\n")
	if c.SkipAuth {
		result.WriteString(fmt.Sprintf("noauth = %t\n\n", c.SkipAuth))
	} else {
		result.WriteString("# noauth = false\n\n")
	}

	result.WriteString("# Application instance ID\n")
	if c.UserProvidedID {
		result.WriteString(fmt.Sprintf("node_id = \"%s\"\n\n", c.ID))
	} else {
		result.WriteString("# node_id = \"<auto-generated at each server start>\"\n\n")
	}

	result.WriteString("# The application secret key\n")

	if c.Secret != "" {
		result.WriteString(fmt.Sprintf("secret = \"%s\"\n\n", c.Secret))
	} else {
		result.WriteString("secret = \"none\"\n\n")
	}

	result.WriteString("# Broadcasting adapters for app-to-clients messages\n")
	result.WriteString(fmt.Sprintf("broadcast_adapters = [\"%s\"]\n\n", strings.Join(c.BroadcastAdapters, "\", \"")))

	result.WriteString("# Broadcasting authorization key\n")

	if c.BroadcastKey != "" { // nolint: gocritic
		result.WriteString(fmt.Sprintf("broadcast_key = \"%s\"\n\n", c.BroadcastKey))
	} else if c.Secret != "" {
		result.WriteString("# broadcast_key = \"<auto-generated from the application secret>\"\n\n")
	} else {
		result.WriteString("broadcast_key = \"none\"\n\n")
	}

	result.WriteString("# Pub/sub adapter for inter-node communication\n")
	if c.PubSubAdapter == "" {
		result.WriteString("# pubsub_adapter = \"redis\" # or \"nats\"\n\n")
	} else {
		result.WriteString(fmt.Sprintf("pubsub_adapter = \"%s\"\n\n", c.PubSubAdapter))
	}

	result.WriteString("# User-provided configuration presets\n")
	if len(c.UserPresets) == 0 {
		result.WriteString("# presets = [\"broker\"]\n\n")
	} else {
		result.WriteString(fmt.Sprintf("presets = [\"%s\"]\n\n", strings.Join(c.UserPresets, "\", \"")))
	}

	result.WriteString("# Server configuration\n[server]\n")
	result.WriteString(c.Server.ToToml())

	result.WriteString("# Logging configuration\n[logging]\n")
	result.WriteString(c.Log.ToToml())

	result.WriteString("# RPC configuration\n[rpc]\n")
	result.WriteString(c.RPC.ToToml())

	result.WriteString("# Broker configuration\n[broker]\n")
	result.WriteString(c.Broker.ToToml())

	result.WriteString("# JWT configuration\n[jwt]\n")
	result.WriteString(c.JWT.ToToml())

	result.WriteString("# Pub/sub (signed) streams configuration\n[streams]\n")
	result.WriteString(c.Streams.ToToml())

	result.WriteString("# WebSockets configuration\n[ws]\n")
	result.WriteString(c.WS.ToToml())

	result.WriteString("# SSE configuration\n[sse]\n")
	result.WriteString(c.SSE.ToToml())

	result.WriteString("# Redis configuration\n[redis]\n")
	result.WriteString(c.Redis.ToToml())

	result.WriteString("# NATS configuration\n[nats]\n")
	result.WriteString(c.NATS.ToToml())

	result.WriteString("# Broadcast adapters configuration\n[http_broadcast]\n")
	result.WriteString(c.HTTPBroadcast.ToToml())
	result.WriteString("[redis_stream_broadcast]\n")
	result.WriteString(c.RedisBroadcast.ToToml())
	result.WriteString("[redis_pubsub_broadcast]\n")
	result.WriteString(c.LegacyRedisBroadcast.ToToml())
	result.WriteString("[nats_broadcast]\n")
	result.WriteString(c.NATSBroadcast.ToToml())

	result.WriteString("# Pub/sub adapters configuration\n[redis_pubsub]\n")
	result.WriteString(c.RedisPubSub.ToToml())
	result.WriteString("[nats_pubsub]\n")
	result.WriteString(c.NATSPubSub.ToToml())

	result.WriteString("# Metrics configuration\n[metrics]\n")
	result.WriteString(c.Metrics.ToToml())

	result.WriteString("# App configuration\n[app]\n")
	result.WriteString(c.App.ToToml())

	result.WriteString("# Disconnector configuration\n[disconnector]\n")
	result.WriteString(c.DisconnectQueue.ToToml())

	result.WriteString("# Embedded NATS configuration\n[embedded_nats]\n")
	result.WriteString(c.EmbeddedNats.ToToml())

	return result.String()
}
