package broadcast

import (
	"context"
	"log/slog"

	"github.com/nats-io/nats.go"

	nconfig "github.com/anycable/anycable-go/nats"
)

type LegacyNATSBroadcaster struct {
	conn    *nats.Conn
	handler Handler
	config  *nconfig.NATSConfig

	log *slog.Logger
}

var _ Broadcaster = (*LegacyNATSBroadcaster)(nil)

func NewLegacyNATSBroadcaster(node Handler, c *nconfig.NATSConfig, l *slog.Logger) *LegacyNATSBroadcaster {
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
		nats.MaxReconnects(s.config.MaxReconnectAttempts),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				s.log.Warn("connection failed", "error", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			s.log.Info("connection restored", "url", nc.ConnectedUrl())
		}),
	}

	if s.config.DontRandomizeServers {
		connectOptions = append(connectOptions, nats.DontRandomize())
	}

	nc, err := nats.Connect(s.config.Servers, connectOptions...)

	if err != nil {
		return err
	}

	_, err = nc.Subscribe(s.config.Channel, func(m *nats.Msg) {
		s.log.Debug("incoming pubsub message", "data", m.Data)
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
