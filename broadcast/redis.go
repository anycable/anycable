package broadcast

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/anycable/anycable-go/redis"
	"github.com/joomcode/errorx"

	nanoid "github.com/matoous/go-nanoid"
)

type RedisConfig struct {
	Stream string `toml:"stream"`
	Group  string `toml:"group"`
	// Redis stream read wait time in milliseconds
	StreamReadBlockMilliseconds int64 `toml:"stream_read_block_milliseconds"`

	Redis *redis.RedisConfig `toml:"redis"`
}

func NewRedisConfig() RedisConfig {
	return RedisConfig{
		Stream:                      "__anycable__",
		Group:                       "bx",
		StreamReadBlockMilliseconds: 2000,
	}
}

func (c RedisConfig) ToToml() string {
	var result strings.Builder

	result.WriteString("# Redis stream name for broadcasts\n")
	result.WriteString(fmt.Sprintf("stream = \"%s\"\n", c.Stream))

	result.WriteString("# Stream consumer group name\n")
	result.WriteString(fmt.Sprintf("group = \"%s\"\n", c.Group))

	result.WriteString("# Streams read wait time in milliseconds\n")
	result.WriteString(fmt.Sprintf("stream_read_block_milliseconds = %d\n", c.StreamReadBlockMilliseconds))

	result.WriteString("\n")

	return result.String()
}

// RedisBroadcaster represents Redis broadcaster using Redis streams
type RedisBroadcaster struct {
	node   Handler
	config *RedisConfig

	streamer *redis.Streamer

	log *slog.Logger
}

var _ Broadcaster = (*RedisBroadcaster)(nil)

// NewRedisBroadcaster builds a new RedisSubscriber struct
func NewRedisBroadcaster(n Handler, c *RedisConfig, l *slog.Logger) *RedisBroadcaster {
	name, _ := nanoid.Nanoid(6)

	log := l.With("context", "broadcast").With("provider", "redisx").With("consumer", name)

	b := RedisBroadcaster{
		node:   n,
		config: c,
		log:    log,
	}

	streamer := redis.NewStreamer(c.Stream, c.Group, c.Redis, log,
		redis.StreamerWithBlockMS(c.StreamReadBlockMilliseconds),
		redis.StreamerWithConsumerName(name),
		redis.StreamerWithHandler(b.handleMessage),
	)

	b.streamer = streamer

	return &b
}

func (s *RedisBroadcaster) IsFanout() bool {
	return false
}

func (s *RedisBroadcaster) Start(done chan error) error {
	if err := s.streamer.Start(); err != nil {
		return errorx.Decorate(err, "failed to initialize Redis stream")
	}

	if s.config.Redis.IsSentinel() { //nolint:gocritic
		s.log.With("stream", s.config.Stream).Info(fmt.Sprintf("Starting Redis broadcaster at %v (sentinels)", s.config.Redis.Hostnames()))
	} else if s.config.Redis.IsCluster() {
		s.log.With("stream", s.config.Stream).Info(fmt.Sprintf("Starting Redis broadcaster at %v (cluster)", s.config.Redis.Hostnames()))
	} else {
		s.log.With("stream", s.config.Stream).Info(fmt.Sprintf("Starting Redis broadcaster at %s", s.config.Redis.Hostname()))
	}

	return nil
}

func (s *RedisBroadcaster) Shutdown(ctx context.Context) error {
	return s.streamer.Shutdown(ctx)
}

func (s *RedisBroadcaster) handleMessage(message map[string]string) error {
	if payload, pok := message["payload"]; pok {
		s.log.Debug("received broadcast")
		return s.node.HandleBroadcast([]byte(payload))
	} else {
		return errors.New("missing payload field")
	}
}
