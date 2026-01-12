package broadcast

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	rconfig "github.com/anycable/anycable-go/redis"
	"github.com/anycable/anycable-go/utils"
	"github.com/redis/rueidis"
)

type LegacyRedisConfig struct {
	Channel string               `toml:"channel"`
	Redis   *rconfig.RedisConfig `toml:"redis"`
}

func NewLegacyRedisConfig() LegacyRedisConfig {
	return LegacyRedisConfig{
		Channel: "__anycable__",
	}
}

func (c LegacyRedisConfig) ToToml() string {
	var result strings.Builder
	result.WriteString(fmt.Sprintf("channel = \"%s\"\n", c.Channel))

	result.WriteString("\n")

	return result.String()
}

// LegacyRedisBroadcaster contains information about Redis pubsub connection
type LegacyRedisBroadcaster struct {
	node   Handler
	config *LegacyRedisConfig

	client           rueidis.Client
	clientOptions    *rueidis.ClientOption
	clientMu         sync.RWMutex
	reconnectAttempt int

	shutdownCh chan struct{}

	log *slog.Logger
}

// NewLegacyRedisBroadcaster returns new LegacyRedisBroadcaster struct
func NewLegacyRedisBroadcaster(node Handler, config *LegacyRedisConfig, l *slog.Logger) *LegacyRedisBroadcaster {
	return &LegacyRedisBroadcaster{
		node:       node,
		config:     config,
		log:        l.With("context", "broadcast").With("provider", "redis"),
		shutdownCh: make(chan struct{}),
	}
}

func (*LegacyRedisBroadcaster) IsFanout() bool {
	return true
}

// Start connects to Redis and subscribes to the pubsub channel
func (s *LegacyRedisBroadcaster) Start(done chan (error)) error {
	options, err := s.config.Redis.ToRueidisOptions()

	if err != nil {
		return err
	}

	s.clientOptions = options

	if s.config.Redis.IsSentinel() { //nolint:gocritic
		s.log.Info(fmt.Sprintf("Starting Redis pub/sub (sentinels): %v", s.config.Redis.Hostnames()))
	} else if s.config.Redis.IsCluster() {
		s.log.Info(fmt.Sprintf("Starting Redis pub/sub (cluster): %v", s.config.Redis.Hostnames()))
	} else {
		s.log.Debug(fmt.Sprintf("Starting Redis pub/sub: %s", s.config.Redis.Hostname()))
	}

	go s.runPubSub(done)

	return nil
}

// Shutdown shuts down the Redis connection
func (s *LegacyRedisBroadcaster) Shutdown(ctx context.Context) error {
	s.clientMu.RLock()
	defer s.clientMu.RUnlock()

	if s.client == nil {
		return nil
	}

	s.log.Debug("shutting down Redis pub/sub")

	close(s.shutdownCh)
	s.client.Close()

	return nil
}

func (s *LegacyRedisBroadcaster) initClient() error {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()

	if s.client != nil {
		return nil
	}

	c, err := rueidis.NewClient(*s.clientOptions)

	if err != nil {
		return err
	}

	s.client = c

	return nil
}

func (s *LegacyRedisBroadcaster) runPubSub(done chan (error)) {
	err := s.initClient()

	if err != nil {
		s.log.Error("failed to connect to Redis", "error", err)
		s.maybeReconnect(done)
		return
	}

	client, cancel := s.client.Dedicate()
	defer cancel()

	s.log.Debug("initialized pub/sub client")

	wait := client.SetPubSubHooks(rueidis.PubSubHooks{
		OnSubscription: func(m rueidis.PubSubSubscription) {
			if m.Kind == "subscribe" && m.Channel == s.config.Channel {
				if s.reconnectAttempt > 0 {
					s.log.With("channel", m.Channel).Info("reconnected to Redis channel")
				} else {
					s.log.With("channel", m.Channel).Info("subscribed to Redis channel")
				}
				s.reconnectAttempt = 0
			}

			s.log.With("channel", m.Channel).Debug(m.Kind)
		},
		OnMessage: func(m rueidis.PubSubMessage) {
			s.log.Debug("received pubsub message")
			s.node.HandlePubSub([]byte(m.Message))
		},
	})

	// Subscribe to the channel
	err = client.Do(context.Background(), client.B().Subscribe().Channel(s.config.Channel).Build()).Error()
	if err != nil {
		s.log.Error("failed to subscribe to Redis channel", "error", err)
		s.maybeReconnect(done)
		return
	}

	for {
		select {
		case err := <-wait:
			if err != nil {
				s.log.Error("Redis subscription error", "error", err)
			}

			s.log.Warn("Redis connection failed", "error", err)
			s.maybeReconnect(done)

			return
		case <-s.shutdownCh:
			s.log.Debug("close pub/sub channel")
			return
		}
	}
}

func (s *LegacyRedisBroadcaster) maybeReconnect(done chan (error)) {
	if s.reconnectAttempt >= s.config.Redis.MaxReconnectAttempts {
		done <- errors.New("Redis reconnect attempts exceeded")
		return
	}

	s.clientMu.Lock()
	if s.client != nil {
		s.client.Close()
		s.client = nil
	}
	s.clientMu.Unlock()

	s.reconnectAttempt++

	delay := utils.NextRetry(s.reconnectAttempt - 1)

	s.log.Info(fmt.Sprintf("next Redis reconnect attempt in %s", delay))
	time.Sleep(delay)

	s.log.Info("reconnecting to Redis...")

	go s.runPubSub(done)
}
