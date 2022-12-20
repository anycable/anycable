package broadcast

import (
	"github.com/apex/log"
	"github.com/nats-io/nats.go"

	nconfig "github.com/anycable/anycable-go/nats"
)

type LegacyNATSBroadcaster struct {
	conn    *nats.Conn
	handler Handler
	config  *nconfig.NATSConfig

	log *log.Entry
}

var _ Broadcaster = (*LegacyNATSBroadcaster)(nil)

func NewLegacyNATSBroadcaster(node Handler, c *nconfig.NATSConfig) *LegacyNATSBroadcaster {
	return &LegacyNATSBroadcaster{
		config:  c,
		handler: node,
		log:     log.WithFields(log.Fields{"context": "pubsub", "provider": "nats"}),
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
				log.Warnf("Connection failed: %v", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Infof("Connection restored: %s", nc.ConnectedUrl())
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
		s.log.Debugf("Incoming pubsub message: %s", m.Data)
		s.handler.HandlePubSub(m.Data)
	})

	if err != nil {
		nc.Close()
		return err
	}

	s.log.Infof("Subscribing for broadcasts to channel: %s", s.config.Channel)

	s.conn = nc

	return nil
}

func (s *LegacyNATSBroadcaster) Shutdown() error {
	if s.conn != nil {
		s.conn.Close()
	}

	return nil
}
