package node

import (
	"context"

	"github.com/apex/log"
)

// Disconnector is an interface for disconnect queue implementation
type Disconnector interface {
	Run() error
	Shutdown(ctx context.Context) error
	Enqueue(*Session) error
	Size() int
}

// NoopDisconnectQueue is non-operational disconnect queue implementation
type NoopDisconnectQueue struct{}

// Run does nothing
func (d *NoopDisconnectQueue) Run() error {
	log.WithField("context", "disconnector").Info("Disconnect events are turned off")
	return nil
}

// Shutdown does nothing
func (d *NoopDisconnectQueue) Shutdown(ctx context.Context) error {
	return nil
}

// Size returns 0
func (d *NoopDisconnectQueue) Size() int {
	return 0
}

// Enqueue does nothing
func (d *NoopDisconnectQueue) Enqueue(s *Session) error {
	return nil
}

// NewNoopDisconnector returns new NoopDisconnectQueue
func NewNoopDisconnector() *NoopDisconnectQueue {
	return &NoopDisconnectQueue{}
}
