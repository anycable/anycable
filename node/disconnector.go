package node

import (
	"context"
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

// InlineDisconnector performs Disconnect calls synchronously
type InlineDisconnector struct {
	n *Node
}

// Run does nothing
func (d *InlineDisconnector) Run() error {
	return nil
}

// Shutdown does nothing
func (d *InlineDisconnector) Shutdown(ctx context.Context) error {
	return nil
}

// Size returns 0
func (d *InlineDisconnector) Size() int {
	return 0
}

// Enqueue disconnects session immediately
func (d *InlineDisconnector) Enqueue(s *Session) error {
	return d.n.DisconnectNow(s)
}

// NewInlineDisconnector returns new InlineDisconnector
func NewInlineDisconnector(n *Node) *InlineDisconnector {
	return &InlineDisconnector{n: n}
}
