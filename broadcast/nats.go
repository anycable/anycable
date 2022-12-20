package broadcast

import (
	"github.com/apex/log"
	"github.com/nats-io/nats.go"
)

type NATSBroadcaster struct {
	conn    *nats.Conn
	handler Handler
	config  *NATSConfig

	log *log.Entry
}

var _ Broadcaster = (*NATSBroadcaster)(nil)

type NATSConfig struct {
	Servers              string
	Channel              string
	DontRandomizeServers bool
}

func NewNATSConfig() NATSConfig {
	return NATSConfig{Servers: nats.DefaultURL, Channel: "__anycable__"}
}

func NewNATSBroadcaster(node Handler, c *NATSConfig) *NATSBroadcaster {
	return &NATSBroadcaster{
		config:  c,
		handler: node,
		log:     log.WithFields(log.Fields{"context": "pubsub", "provider": "nats"}),
	}
}

func (s *NATSBroadcaster) Start(done chan (error)) error {
	connectOptions := []nats.Option{
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(maxReconnectAttempts),
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

func (s *NATSBroadcaster) Shutdown() error {
	if s.conn != nil {
		s.conn.Close()
	}

	return nil
}
