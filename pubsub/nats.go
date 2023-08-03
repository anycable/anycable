package pubsub

import (
	"context"
	"sync"

	"github.com/anycable/anycable-go/common"
	nconfig "github.com/anycable/anycable-go/nats"
	"github.com/anycable/anycable-go/utils"

	"github.com/apex/log"
	"github.com/nats-io/nats.go"
)

type NATSSubscriber struct {
	node   Handler
	config *nconfig.NATSConfig

	conn *nats.Conn

	subscriptions map[string]*nats.Subscription
	subMu         sync.RWMutex

	log *log.Entry
}

var _ Subscriber = (*NATSSubscriber)(nil)

// NewNATSSubscriber creates a NATS subscriber using pub/sub
func NewNATSSubscriber(node Handler, config *nconfig.NATSConfig) (*NATSSubscriber, error) {
	return &NATSSubscriber{
		node:          node,
		config:        config,
		subscriptions: make(map[string]*nats.Subscription),
		log:           log.WithField("context", "pubsub"),
	}, nil
}

func (s *NATSSubscriber) Start(done chan (error)) error {
	connectOptions := []nats.Option{
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(s.config.MaxReconnectAttempts),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				s.log.Warnf("Connection failed: %v", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			s.log.Infof("Connection restored: %s", nc.ConnectedUrl())
		}),
	}

	if s.config.DontRandomizeServers {
		connectOptions = append(connectOptions, nats.DontRandomize())
	}

	nc, err := nats.Connect(s.config.Servers, connectOptions...)

	if err != nil {
		return err
	}

	s.log.Infof("Starting NATS pub/sub: %s", s.config.Servers)

	s.conn = nc

	s.Subscribe(s.config.InternalChannel)

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
		s.log.Errorf("Failed to subscribe to %s: %v", stream, err)
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
	s.Publish(s.config.InternalChannel, cmd)
}

func (s *NATSSubscriber) Publish(stream string, msg interface{}) {
	s.log.WithField("channel", stream).Debugf("Publish message: %v", msg)

	if err := s.conn.Publish(stream, utils.ToJSON(msg)); err != nil {
		s.log.Errorf("Failed to publish message: %v", err)
	}
}

func (s *NATSSubscriber) handleMessage(m *nats.Msg) {
	s.log.WithField("channel", m.Subject).Debugf("Received message: %v", m.Data)

	msg, err := common.PubSubMessageFromJSON(m.Data)

	if err != nil {
		s.log.Warnf("Failed to parse pubsub message '%s' with error: %v", m.Data, err)
		return
	}

	switch v := msg.(type) {
	case common.StreamMessage:
		s.node.Broadcast(&v)
	case common.RemoteCommandMessage:
		s.node.ExecuteRemoteCommand(&v)
	}
}
