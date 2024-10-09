package pubsub

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/logger"
	nconfig "github.com/anycable/anycable-go/nats"
	"github.com/anycable/anycable-go/utils"

	"github.com/nats-io/nats.go"
)

type NATSConfig struct {
	Channel string `toml:"channel"`
	NATS    *nconfig.NATSConfig
}

func NewNATSConfig() NATSConfig {
	return NATSConfig{
		Channel: "__anycable_internal__",
	}
}

func (c NATSConfig) ToToml() string {
	var result strings.Builder
	result.WriteString(fmt.Sprintf("channel = \"%s\"\n", c.Channel))

	result.WriteString("\n")

	return result.String()
}

type NATSSubscriber struct {
	node   Handler
	config *NATSConfig

	conn *nats.Conn

	subscriptions map[string]*nats.Subscription
	subMu         sync.RWMutex

	log *slog.Logger
}

var _ Subscriber = (*NATSSubscriber)(nil)

// NewNATSSubscriber creates a NATS subscriber using pub/sub
func NewNATSSubscriber(node Handler, config *NATSConfig, l *slog.Logger) (*NATSSubscriber, error) {
	return &NATSSubscriber{
		node:          node,
		config:        config,
		subscriptions: make(map[string]*nats.Subscription),
		log:           l.With("context", "pubsub"),
	}, nil
}

func (s *NATSSubscriber) Start(done chan (error)) error {
	connectOptions := []nats.Option{
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(s.config.NATS.MaxReconnectAttempts),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				s.log.Warn("connection failed", "error", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			s.log.Info("connection restored", "url", nc.ConnectedUrl())
		}),
	}

	if s.config.NATS.DontRandomizeServers {
		connectOptions = append(connectOptions, nats.DontRandomize())
	}

	nc, err := nats.Connect(s.config.NATS.Servers, connectOptions...)

	if err != nil {
		return err
	}

	s.log.Info(fmt.Sprintf("Starting NATS pub/sub: %s", s.config.NATS.Servers))

	s.conn = nc

	s.Subscribe(s.config.Channel)

	return nil
}

func (s *NATSSubscriber) Shutdown(ctx context.Context) error {
	if s.conn != nil {
		s.conn.Close()
	}

	return nil
}

func (s *NATSSubscriber) IsMultiNode() bool {
	return true
}

func (s *NATSSubscriber) Subscribe(stream string) {
	s.subMu.RLock()
	if _, ok := s.subscriptions[stream]; ok {
		s.subMu.RUnlock()
		return
	}

	s.subMu.RUnlock()

	s.subMu.Lock()
	defer s.subMu.Unlock()

	sub, err := s.conn.Subscribe(stream, s.handleMessage)

	if err != nil {
		s.log.Error("failed to subscribe", "stream", stream, "error", err)
		return
	}

	s.subscriptions[stream] = sub
}

func (s *NATSSubscriber) Unsubscribe(stream string) {
	s.subMu.Lock()
	defer s.subMu.Unlock()

	if sub, ok := s.subscriptions[stream]; ok {
		delete(s.subscriptions, stream)
		sub.Unsubscribe() // nolint:errcheck
	}
}

func (s *NATSSubscriber) Broadcast(msg *common.StreamMessage) {
	s.Publish(msg.Stream, msg)
}

func (s *NATSSubscriber) BroadcastCommand(cmd *common.RemoteCommandMessage) {
	s.Publish(s.config.Channel, cmd)
}

func (s *NATSSubscriber) Publish(stream string, msg interface{}) {
	s.log.With("channel", stream).Debug("publish message", "data", msg)

	if err := s.conn.Publish(stream, utils.ToJSON(msg)); err != nil {
		s.log.Error("failed to publish message", "error", err)
	}
}

func (s *NATSSubscriber) handleMessage(m *nats.Msg) {
	msg, err := common.PubSubMessageFromJSON(m.Data)

	if err != nil {
		s.log.Warn("failed to parse pubsub message", "data", logger.CompactValue(m.Data), "error", err)
		return
	}

	switch v := msg.(type) {
	case common.StreamMessage:
		s.log.With("channel", m.Subject).Debug("received broadcast message")
		s.node.Broadcast(&v)
	case common.RemoteCommandMessage:
		s.log.With("channel", m.Subject).Debug("received remote command")
		s.node.ExecuteRemoteCommand(&v)
	default:
		s.log.With("channel", m.Subject).Warn("received unknown message", "data", logger.CompactValue(m.Data))
	}
}
