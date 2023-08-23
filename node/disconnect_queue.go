package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/apex/log"
)

// DisconnectQueueConfig contains DisconnectQueue configuration
type DisconnectQueueConfig struct {
	// Limit the number of Disconnect RPC calls per second
	Rate int
	// The size of the channel's buffer for disconnect requests
	Backlog int
	// How much time wait to call all enqueued calls at exit (in seconds) [DEPREACTED]
	ShutdownTimeout int
}

// NewDisconnectQueueConfig builds a new config
func NewDisconnectQueueConfig() DisconnectQueueConfig {
	return DisconnectQueueConfig{Rate: 100, Backlog: 4096}
}

// DisconnectQueue is a rate-limited executor
type DisconnectQueue struct {
	node *Node
	// Throttling rate
	rate time.Duration
	// Call RPC Disconnect for connections
	disconnect chan *Session
	// Logger with context
	log *log.Entry
	// Control channel to shutdown the executer
	shutdown chan struct{}
	// Executer stopped status
	isStopped bool
	// Mutex to work with stopped status concurrently
	mu sync.Mutex
}

// NewDisconnectQueue builds new queue with a specified rate (max calls per second)
func NewDisconnectQueue(node *Node, config *DisconnectQueueConfig) *DisconnectQueue {
	rateDuration := time.Millisecond * time.Duration(1000/config.Rate)

	ctx := log.WithField("context", "disconnector")

	ctx.Debugf("Calls rate: %v", rateDuration)

	return &DisconnectQueue{
		node:       node,
		disconnect: make(chan *Session, config.Backlog),
		rate:       rateDuration,
		log:        ctx,
		shutdown:   make(chan struct{}, 1),
	}
}

// Run starts queue
func (d *DisconnectQueue) Run() error {
	throttle := time.NewTicker(d.rate)
	defer throttle.Stop()

	for {
		select {
		case session := <-d.disconnect:
			<-throttle.C
			d.node.DisconnectNow(session) //nolint:errcheck
		case <-d.shutdown:
			return nil
		}
	}
}

// Shutdown stops throttling and makes requests one by one
func (d *DisconnectQueue) Shutdown(ctx context.Context) error {
	d.mu.Lock()
	if d.isStopped {
		d.mu.Unlock()
		return nil
	}

	d.isStopped = true
	d.shutdown <- struct{}{}
	d.mu.Unlock()

	left := len(d.disconnect)
	actual := 0

	if left == 0 {
		return nil
	}

	defer func() {
		d.log.Infof("Disconnected %d sessions", actual)
	}()

	deadline, ok := ctx.Deadline()

	if ok {
		timeLeft := time.Until(deadline)

		d.log.Infof("Invoking remaining disconnects for %2fs: ~%d", timeLeft.Seconds(), left)
	} else {
		d.log.Infof("Invoking remaining disconnects: ~%d", left)
	}

	for {
		select {
		case session := <-d.disconnect:
			d.node.DisconnectNow(session) // nolint:errcheck

			actual++
		case <-ctx.Done():
			return fmt.Errorf("Had no time to invoke Disconnect calls: ~%d", len(d.disconnect))
		default:
			return nil
		}
	}
}

// Enqueue adds session to the disconnect queue
func (d *DisconnectQueue) Enqueue(s *Session) error {
	d.mu.Lock()

	// Check that we're not closed
	if d.isStopped {
		d.mu.Unlock()
		return nil
	}

	d.mu.Unlock()

	d.disconnect <- s

	return nil
}

// Size returns the number of enqueued tasks
func (d *DisconnectQueue) Size() int {
	return len(d.disconnect)
}
