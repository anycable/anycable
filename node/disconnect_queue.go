package node

import (
	"fmt"
	"sync"
	"time"

	"github.com/apex/log"
)

// DisconnectQueueConfig contains DisconnectQueue configuration
type DisconnectQueueConfig struct {
	// Limit the number of Disconnect RPC calls per second
	Rate int
	// How much time wait to call all enqueued calls at exit (in seconds)
	ShutdownTimeout int
}

// NewDisconnectQueueConfig builds a new config
func NewDisconnectQueueConfig() DisconnectQueueConfig {
	return DisconnectQueueConfig{ShutdownTimeout: 5, Rate: 100}
}

// DisconnectQueue is a rate-limited executor
type DisconnectQueue struct {
	node *Node
	// Throttling rate
	rate time.Duration
	// Graceful shutdown timeout
	timeout time.Duration
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
	timeout := time.Duration(config.ShutdownTimeout) * time.Second

	ctx := log.WithField("context", "disconnector")

	ctx.Debugf("Calls rate: %v", rateDuration)

	return &DisconnectQueue{
		node:       node,
		disconnect: make(chan *Session, 4096),
		rate:       rateDuration,
		timeout:    timeout,
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
			d.node.DisconnectNow(session)
		case <-d.shutdown:
			return nil
		}
	}
}

// Shutdown stops throttling and makes requests one by one
func (d *DisconnectQueue) Shutdown() error {
	d.mu.Lock()
	if d.isStopped {
		d.mu.Unlock()
		return nil
	}

	d.isStopped = true
	d.shutdown <- struct{}{}
	d.mu.Unlock()

	left := len(d.disconnect)

	if left == 0 {
		return nil
	}

	d.log.Infof("Invoking remaining disconnects for %s: %d", d.timeout, left)

	for {
		select {
		case session := <-d.disconnect:
			err := d.node.DisconnectNow(session)

			left--

			if err != nil {
				return err
			}

			if left == 0 {
				return nil
			}
		case <-time.After(d.timeout):
			return fmt.Errorf("Had no time to invoke Disconnect calls: %d", len(d.disconnect))
		}
	}
}

// Enqueue adds session to the disconnect queue
func (d *DisconnectQueue) Enqueue(s *Session) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check that we're not closed
	if d.isStopped {
		return nil
	}

	d.disconnect <- s

	return nil
}

// Size returns the number of enqueued tasks
func (d *DisconnectQueue) Size() int {
	return len(d.disconnect)
}
