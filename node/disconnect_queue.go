package node

import (
	"fmt"
	"sync"
	"time"

	"github.com/apex/log"
)

const (
	// How much time wait to call all enqueued calls
	// TODO: make configurable
	waitTime = 5 * time.Second
)

// DisconnectQueue is a rate-limited executor
type DisconnectQueue struct {
	node *Node
	// Limit the number of RPC calls per second
	rate time.Duration
	// Call RPC Disconnect for connections
	disconnect chan *Session
	// Logger with context
	log        *log.Entry
	shutdownCh chan bool
	mu         sync.Mutex
}

// NewDisconnectQueue builds new queue with a specified rate (max calls per second)
func NewDisconnectQueue(node *Node, rate int) *DisconnectQueue {
	rateDuration := time.Millisecond * time.Duration(1000/rate)

	ctx := log.WithField("context", "disconnector")

	ctx.Debugf("Calls rate: %v", rateDuration)

	return &DisconnectQueue{
		node:       node,
		disconnect: make(chan *Session, 128),
		rate:       rateDuration,
		log:        ctx,
		shutdownCh: make(chan bool),
	}
}

// Run starts queue
func (d *DisconnectQueue) Run() {
	throttle := time.Tick(d.rate)

	for {
		select {
		case session := <-d.disconnect:
			<-throttle
			d.node.DisconnectNow(session)
		case <-d.shutdownCh:
			d.mu.Lock()
			defer d.mu.Unlock()
			if d.shutdownCh != nil {
				close(d.shutdownCh)
				d.shutdownCh = nil
			}
			return
		}
	}
}

// Shutdown stops throttling and makes requests one by one
// for
func (d *DisconnectQueue) Shutdown() error {
	d.shutdownCh <- true

	left := len(d.disconnect)
	defer close(d.disconnect)

	if left == 0 {
		return nil
	}

	d.log.Infof("Invoking remaining disconnects: %d", left)

	for {
		select {
		case session := <-d.disconnect:
			err := d.node.DisconnectNow(session)

			left--

			if err != nil {
				return err
			}
		case <-time.After(waitTime):
			return fmt.Errorf("Had no time to invoke Disconnect calls: %d", len(d.disconnect))
		}
	}
}

// Enqueue adds session to the disconnect queue
func (d *DisconnectQueue) Enqueue(s *Session) {
	d.mu.Lock()
	// Check that we're not closed
	if d.shutdownCh == nil {
		return
	}
	d.mu.Unlock()

	d.disconnect <- s
}

// Size returns the number of enqueued tasks
func (d *DisconnectQueue) Size() int {
	return len(d.disconnect)
}
