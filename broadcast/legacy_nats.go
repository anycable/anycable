package broadcast

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nats-io/nats.go"

	nconfig "github.com/anycable/anycable-go/nats"
)

type LegacyNATSConfig struct {
	Channel string              `toml:"channel"`
	NATS    *nconfig.NATSConfig `toml:"nats"`
}

func NewLegacyNATSConfig() LegacyNATSConfig {
	return LegacyNATSConfig{
		Channel: "__anycable__",
	}
}

func (c LegacyNATSConfig) ToToml() string {
	var result strings.Builder
	result.WriteString(fmt.Sprintf("channel = \"%s\"\n", c.Channel))

	result.WriteString("\n")

	return result.String()
}

type LegacyNATSBroadcaster struct {
	conn    *nats.Conn
	handler Handler
	config  *LegacyNATSConfig

	log *slog.Logger
}

var _ Broadcaster = (*LegacyNATSBroadcaster)(nil)

func NewLegacyNATSBroadcaster(node Handler, c *LegacyNATSConfig, l *slog.Logger) *LegacyNATSBroadcaster {
	return &LegacyNATSBroadcaster{
		config:  c,
		handler: node,
		log:     l.With("context", "broadcast").With("provider", "nats"),
	}
}

func (LegacyNATSBroadcaster) IsFanout() bool {
	return true
}

func (s *LegacyNATSBroadcaster) Start(done chan (error)) error {
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

	_, err = nc.Subscribe(s.config.Channel, func(m *nats.Msg) {
		s.log.Debug("received pubsub message")
		s.handler.HandlePubSub(m.Data)
	})

	if err != nil {
		nc.Close()
		return err
	}

	s.log.Info("subscribing for broadcasts", "channel", s.config.Channel)

	s.conn = nc

	return nil
}

func (s *LegacyNATSBroadcaster) Shutdown(ctx context.Context) error {
	if s.conn != nil {
		s.conn.Close()
	}

	return nil
}
