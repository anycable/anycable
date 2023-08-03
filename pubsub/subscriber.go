package pubsub

import (
	"context"

	"github.com/anycable/anycable-go/common"
)

// Subscriber is responsible for subscribing to individual streams and
// and publishing messages to streams
//
//go:generate mockery --name Subscriber --output "../mocks" --outpkg mocks
type Subscriber interface {
	Start(done chan (error)) error
	Shutdown(ctx context.Context) error
	Broadcast(msg *common.StreamMessage)
	BroadcastCommand(msg *common.RemoteCommandMessage)
	Subscribe(stream string)
	Unsubscribe(stream string)
	IsMultiNode() bool
}

type Handler interface {
	Broadcast(msg *common.StreamMessage)
	ExecuteRemoteCommand(msg *common.RemoteCommandMessage)
}

type LegacySubscriber struct {
	node Handler
}

var _ Subscriber = (*LegacySubscriber)(nil)

// NewLegacySubscriber creates a legacy subscriber implementation to work with legacy Redis and NATS broadcasters
func NewLegacySubscriber(node Handler) *LegacySubscriber {
	return &LegacySubscriber{node: node}
}

func (LegacySubscriber) Start(done chan (error)) error {
	return nil
}

func (LegacySubscriber) Shutdown(ctx context.Context) error {
	return nil
}

func (LegacySubscriber) Subscribe(stream string) {
}

func (LegacySubscriber) Unsubscribe(stream string) {
}

func (s *LegacySubscriber) Broadcast(msg *common.StreamMessage) {
	s.node.Broadcast(msg)
}

func (s *LegacySubscriber) BroadcastCommand(cmd *common.RemoteCommandMessage) {
	s.node.ExecuteRemoteCommand(cmd)
}

func (s *LegacySubscriber) IsMultiNode() bool {
	return false
}
