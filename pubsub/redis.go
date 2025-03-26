package pubsub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/logger"
	rconfig "github.com/anycable/anycable-go/redis"
	"github.com/anycable/anycable-go/utils"
	"github.com/redis/rueidis"
	"golang.org/x/exp/maps"
)

type subscriptionCmd = int

const (
	subscribeCmd subscriptionCmd = iota
	unsubscribeCmd
)

type subscriptionEntry struct {
	id string
}

type RedisConfig struct {
	Channel string `toml:"channel"`
	Redis   *rconfig.RedisConfig
}

func NewRedisConfig() RedisConfig {
	return RedisConfig{
		Channel: "__anycable_internal__",
	}
}

func (c RedisConfig) ToToml() string {
	var result strings.Builder
	result.WriteString(fmt.Sprintf("channel = \"%s\"\n", c.Channel))

	result.WriteString("\n")

	return result.String()
}

type RedisSubscriber struct {
	node   Handler
	config *RedisConfig

	client           rueidis.Client
	clientOptions    *rueidis.ClientOption
	clientMu         sync.RWMutex
	reconnectAttempt int

	subscriptions map[string]*subscriptionEntry
	subMu         sync.RWMutex

	psClient rueidis.DedicatedClient
	psMu     sync.RWMutex

	shutdownCh chan struct{}

	log *slog.Logger

	// test-only
	// TODO: refactor tests to not depend on internals
	events         map[string]subscriptionCmd
	eventsMu       sync.Mutex
	trackingEvents bool
}

var _ Subscriber = (*RedisSubscriber)(nil)

// NewRedisSubscriber creates a Redis subscriber using pub/sub
func NewRedisSubscriber(node Handler, config *RedisConfig, l *slog.Logger) (*RedisSubscriber, error) {
	options, err := config.Redis.ToRueidisOptions()

	if err != nil {
		return nil, err
	}

	return &RedisSubscriber{
		node:           node,
		config:         config,
		clientOptions:  options,
		subscriptions:  make(map[string]*subscriptionEntry),
		log:            l.With("context", "pubsub"),
		shutdownCh:     make(chan struct{}),
		trackingEvents: false,
		events:         make(map[string]subscriptionCmd),
	}, nil
}

func (s *RedisSubscriber) Start(done chan (error)) error {
	if s.config.Redis.IsSentinel() { //nolint:gocritic
		s.log.Info(fmt.Sprintf("Starting Redis pub/sub (sentinels): %v", s.config.Redis.Hostnames()))
	} else if s.config.Redis.IsCluster() {
		s.log.Info(fmt.Sprintf("Starting Redis pub/sub (cluster): %v", s.config.Redis.Hostnames()))
	} else {
		s.log.Info(fmt.Sprintf("Starting Redis pub/sub: %s", s.config.Redis.Hostname()))
	}

	// Add internal channel to subscriptions
	s.subMu.Lock()
	s.subscriptions[s.config.Channel] = &subscriptionEntry{id: s.config.Channel}
	s.subMu.Unlock()

	go s.runPubSub(done)

	return nil
}

func (s *RedisSubscriber) Shutdown(ctx context.Context) error {
	s.clientMu.RLock()
	defer s.clientMu.RUnlock()

	if s.client == nil {
		return nil
	}

	s.log.Debug("shutting down Redis pub/sub")

	// First, shutdown the pub/sub routine
	close(s.shutdownCh)
	s.client.Close()

	return nil
}

func (s *RedisSubscriber) IsMultiNode() bool {
	return true
}

func (s *RedisSubscriber) Subscribe(stream string) {
	s.subMu.Lock()
	s.subscriptions[stream] = &subscriptionEntry{id: stream}
	entry := s.subscriptions[stream]
	s.subMu.Unlock()

	client := s.pubsubClient()
	if client == nil {
		return
	}

	ctx := context.Background()
	s.log.With("channel", entry.id).Debug("subscribing")
	client.Do(ctx, client.B().Subscribe().Channel(entry.id).Build())
}

func (s *RedisSubscriber) Unsubscribe(stream string) {
	s.subMu.Lock()
	if _, ok := s.subscriptions[stream]; !ok {
		s.subMu.Unlock()
		return
	}

	delete(s.subscriptions, stream)
	s.subMu.Unlock()

	client := s.pubsubClient()
	if client == nil {
		return
	}

	ctx := context.Background()
	s.log.With("channel", stream).Debug("unsubscribing")
	client.Do(ctx, client.B().Unsubscribe().Channel(stream).Build())
}

func (s *RedisSubscriber) Broadcast(msg *common.StreamMessage) {
	s.Publish(msg.Stream, msg)
}

func (s *RedisSubscriber) BroadcastCommand(cmd *common.RemoteCommandMessage) {
	s.Publish(s.config.Channel, cmd)
}

func (s *RedisSubscriber) Publish(stream string, msg interface{}) {
	s.clientMu.RLock()

	if s.client == nil {
		s.clientMu.RUnlock()
		return
	}

	ctx := context.Background()
	client := s.client

	s.clientMu.RUnlock()

	s.log.With("channel", stream).Debug("publish message", "data", msg)

	client.Do(ctx, client.B().Publish().Channel(stream).Message(string(utils.ToJSON(msg))).Build())
}

func (s *RedisSubscriber) initClient() error {
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

func (s *RedisSubscriber) pubsubClient() rueidis.DedicatedClient {
	s.psMu.RLock()
	defer s.psMu.RUnlock()

	return s.psClient
}

func (s *RedisSubscriber) runPubSub(done chan (error)) {
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
					s.log.Info("reconnected")
				} else {
					s.log.Info("connected")
				}
				s.reconnectAttempt = 0
			}

			s.log.With("channel", m.Channel).Debug(m.Kind)
			s.trackEvent(m.Kind, m.Channel)
		},
		OnMessage: func(m rueidis.PubSubMessage) {
			msg, err := common.PubSubMessageFromJSON([]byte(m.Message))

			if err != nil {
				s.log.Warn("failed to parse pubsub message", "data", logger.CompactValue(m.Message), "error", err)
				return
			}

			switch v := msg.(type) {
			case common.StreamMessage:
				s.log.With("channel", m.Channel).Debug("received broadcast message")
				s.node.Broadcast(&v)
			case common.RemoteCommandMessage:
				s.log.With("channel", m.Channel).Debug("received remote command")
				s.node.ExecuteRemoteCommand(&v)
			}
		},
	})

	s.psMu.Lock()
	s.psClient = client
	s.psMu.Unlock()

	s.resubscribe(client)

	for {
		select {
		case err := <-wait:
			if err != nil {
				s.log.Error("Redis pub/sub disconnected", "error", err)
			}

			s.maybeReconnect(done)

			return
		case <-s.shutdownCh:
			s.log.Debug("close pub/sub channel")
			return
		}
	}
}

func (s *RedisSubscriber) maybeReconnect(done chan (error)) {
	if s.reconnectAttempt >= s.config.Redis.MaxReconnectAttempts {
		done <- errors.New("failed to reconnect to Redis: attempts exceeded") //nolint:stylecheck
		return
	}

	s.psMu.Lock()
	s.psClient = nil
	s.psMu.Unlock()

	s.clientMu.RLock()
	if s.client != nil {
		// Make sure client knows about connection failure,
		// so the next attempt to Publish won't fail
		s.client.Do(context.Background(), s.client.B().Arbitrary("ping").Build())
	}
	s.clientMu.RUnlock()

	s.reconnectAttempt++

	delay := utils.NextRetry(s.reconnectAttempt - 1)

	s.log.Info(fmt.Sprintf("next Redis reconnect attempt in %s", delay))
	time.Sleep(delay)

	s.log.Info("reconnecting to Redis...")

	go s.runPubSub(done)
}

const batchSubscribeSize = 256

func (s *RedisSubscriber) resubscribe(client rueidis.DedicatedClient) {
	s.subMu.RLock()
	channels := maps.Keys(s.subscriptions)
	s.subMu.RUnlock()

	batch := make([]string, 0, batchSubscribeSize)

	for i, id := range channels {
		if i > 0 && i%batchSubscribeSize == 0 {
			err := batchSubscribe(client, batch)
			if err != nil {
				s.log.Error("failed to resubscribe", "error", err)
				return
			}
			batch = batch[:0]
		}

		batch = append(batch, id)
	}

	if len(batch) > 0 {
		err := batchSubscribe(client, batch)
		if err != nil {
			s.log.Error("failed to resubscribe", "error", err)
			return
		}
	}
}

func batchSubscribe(client rueidis.DedicatedClient, channels []string) error {
	if len(channels) == 0 {
		return nil
	}

	return client.Do(context.Background(), client.B().Subscribe().Channel(channels...).Build()).Error()
}

// test-only
func (s *RedisSubscriber) trackEvent(event string, channel string) {
	if !s.trackingEvents {
		return
	}

	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()

	if event == "subscribe" {
		s.events[channel] = subscribeCmd
	} else if event == "unsubscribe" {
		s.events[channel] = unsubscribeCmd
	}
}

// test-only
func (s *RedisSubscriber) getEvent(channel string) subscriptionCmd {
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()

	cmd, ok := s.events[channel]

	if !ok {
		return unsubscribeCmd
	}

	return cmd
}
