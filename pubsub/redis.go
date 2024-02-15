package pubsub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/anycable/anycable-go/common"
	rconfig "github.com/anycable/anycable-go/redis"
	"github.com/anycable/anycable-go/utils"
	"github.com/redis/rueidis"
)

type subscriptionState = int

const (
	subscriptionPending subscriptionState = iota
	subscriptionCreated
	subscriptionPendingUnsubscribe
)

type subscriptionEntry struct {
	id    string
	state subscriptionState
}

type RedisSubscriber struct {
	node   Handler
	config *rconfig.RedisConfig

	client           rueidis.Client
	clientOptions    *rueidis.ClientOption
	clientMu         sync.RWMutex
	reconnectAttempt int

	subscriptions map[string]*subscriptionEntry
	subMu         sync.RWMutex

	streamsCh  chan (*subscriptionEntry)
	shutdownCh chan struct{}

	log *slog.Logger
}

var _ Subscriber = (*RedisSubscriber)(nil)

// NewRedisSubscriber creates a Redis subscriber using pub/sub
func NewRedisSubscriber(node Handler, config *rconfig.RedisConfig, l *slog.Logger) (*RedisSubscriber, error) {
	options, err := config.ToRueidisOptions()

	if err != nil {
		return nil, err
	}

	return &RedisSubscriber{
		node:          node,
		config:        config,
		clientOptions: options,
		subscriptions: make(map[string]*subscriptionEntry),
		log:           l.With("context", "pubsub"),
		streamsCh:     make(chan *subscriptionEntry, 1024),
		shutdownCh:    make(chan struct{}),
	}, nil
}

func (s *RedisSubscriber) Start(done chan (error)) error {
	if s.config.IsSentinel() { //nolint:gocritic
		s.log.Info(fmt.Sprintf("Starting Redis pub/sub (sentinels): %v", s.config.Hostnames()))
	} else if s.config.IsCluster() {
		s.log.Info(fmt.Sprintf("Starting Redis pub/sub (cluster): %v", s.config.Hostnames()))
	} else {
		s.log.Info(fmt.Sprintf("Starting Redis pub/sub: %s", s.config.Hostname()))
	}

	go s.runPubSub(done)

	s.Subscribe(s.config.InternalChannel)

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
	s.subscriptions[stream] = &subscriptionEntry{state: subscriptionPending, id: stream}
	entry := s.subscriptions[stream]
	s.subMu.Unlock()

	s.streamsCh <- entry
}

func (s *RedisSubscriber) Unsubscribe(stream string) {
	s.subMu.Lock()
	if _, ok := s.subscriptions[stream]; !ok {
		s.subMu.Unlock()
		return
	}

	entry := s.subscriptions[stream]
	entry.state = subscriptionPendingUnsubscribe

	s.streamsCh <- entry
	s.subMu.Unlock()
}

func (s *RedisSubscriber) Broadcast(msg *common.StreamMessage) {
	s.Publish(msg.Stream, msg)
}

func (s *RedisSubscriber) BroadcastCommand(cmd *common.RemoteCommandMessage) {
	s.Publish(s.config.InternalChannel, cmd)
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

func (s *RedisSubscriber) runPubSub(done chan (error)) {
	err := s.initClient()

	if err != nil {
		s.log.Error("failed to connect to Redis", "error", err)
		s.maybeReconnect(done)
		return
	}

	client, cancel := s.client.Dedicate()
	defer cancel()

	wait := client.SetPubSubHooks(rueidis.PubSubHooks{
		OnSubscription: func(m rueidis.PubSubSubscription) {
			s.subMu.Lock()
			defer s.subMu.Unlock()

			s.log.Debug("subscription message", "data", m)

			if m.Kind == "subscribe" && m.Channel == s.config.InternalChannel {
				if s.reconnectAttempt > 0 {
					s.log.Info("reconnected to Redis")
				}
				s.reconnectAttempt = 0
			}

			if entry, ok := s.subscriptions[m.Channel]; ok {
				if entry.state == subscriptionPending && m.Kind == "subscribe" {
					s.log.With("channel", m.Channel).Debug("subscribed")
					entry.state = subscriptionCreated
				}

				if entry.state == subscriptionPendingUnsubscribe && m.Kind == "unsubscribe" {
					s.log.With("channel", m.Channel).Debug("unsubscribed")
					delete(s.subscriptions, entry.id)
				}
			}
		},
		OnMessage: func(m rueidis.PubSubMessage) {
			s.log.With("channel", m.Channel).Debug("received message", "data", m.Message)

			msg, err := common.PubSubMessageFromJSON([]byte(m.Message))

			if err != nil {
				s.log.Warn("failed to parse pubsub message", "data", m.Message, "error", err)
				return
			}

			switch v := msg.(type) {
			case common.StreamMessage:
				s.node.Broadcast(&v)
			case common.RemoteCommandMessage:
				s.node.ExecuteRemoteCommand(&v)
			}
		},
	})

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
		case entry := <-s.streamsCh:
			ctx := context.Background()

			switch entry.state {
			case subscriptionPending:
				s.log.With("channel", entry.id).Debug("subscribing")
				client.Do(ctx, client.B().Subscribe().Channel(entry.id).Build())
			case subscriptionPendingUnsubscribe:
				s.log.With("channel", entry.id).Debug("unsubscribing")
				client.Do(ctx, client.B().Unsubscribe().Channel(entry.id).Build())
			}
		}
	}
}

func (s *RedisSubscriber) subscriptionEntry(stream string) *subscriptionEntry {
	s.subMu.RLock()
	defer s.subMu.RUnlock()

	if entry, ok := s.subscriptions[stream]; ok {
		return entry
	}

	return nil
}

func (s *RedisSubscriber) maybeReconnect(done chan (error)) {
	if s.reconnectAttempt >= s.config.MaxReconnectAttempts {
		done <- errors.New("failed to reconnect to Redis: attempts exceeded") //nolint:stylecheck
		return
	}

	s.clientMu.RLock()
	if s.client != nil {
		// Make sure client knows about connection failure,
		// so the next attempt to Publish won't fail
		s.client.Do(context.Background(), s.client.B().Arbitrary("ping").Build())
	}
	s.clientMu.RUnlock()

	s.subMu.Lock()
	toRemove := []string{}

	for key, sub := range s.subscriptions {
		if sub.state == subscriptionCreated {
			sub.state = subscriptionPending
		}

		if sub.state == subscriptionPendingUnsubscribe {
			toRemove = append(toRemove, key)
		}
	}

	for _, key := range toRemove {
		delete(s.subscriptions, key)
	}
	s.subMu.Unlock()

	s.reconnectAttempt++

	delay := utils.NextRetry(s.reconnectAttempt - 1)

	s.log.Info(fmt.Sprintf("next Redis reconnect attempt in %s", delay))
	time.Sleep(delay)

	s.log.Info("reconnecting to Redis...")

	go s.runPubSub(done)

	s.subMu.RLock()
	defer s.subMu.RUnlock()

	for _, sub := range s.subscriptions {
		if sub.state == subscriptionPending {
			s.log.Debug("resubscribing to stream", "stream", sub.id)
			s.streamsCh <- sub
		}
	}
}
