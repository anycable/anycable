package node

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/anycable/anycable-go/utils"
)

// DisconnectQueueConfig contains DisconnectQueue configuration
type DisconnectQueueConfig struct {
	// Limit the number of Disconnect RPC calls per second
	Rate int
	// The size of the channel's buffer for disconnect requests [DEPREACTED]
	Backlog int
	// The max number of waiting Disconnect calls in the queue; when it's hit, new items are discarded with warning
	MaxBacklog int `toml:"max_backlog"`
	// How much time wait to call all enqueued calls at exit (in seconds) [DEPREACTED]
	ShutdownTimeout int
}

// NewDisconnectQueueConfig builds a new config
func NewDisconnectQueueConfig() DisconnectQueueConfig {
	return DisconnectQueueConfig{Rate: 100, MaxBacklog: 8192}
}

func (c DisconnectQueueConfig) ToToml() string {
	var result strings.Builder

	result.WriteString("# Limit the number of Disconnect RPC calls per second\n")
	result.WriteString(fmt.Sprintf("rate = %d\n", c.Rate))

	result.WriteString("# Max queue size for disconnect requests (new items are discarded when the limit is reached)\n")
	result.WriteString(fmt.Sprintf("max_backlog = %d\n", c.MaxBacklog))

	result.WriteString("\n")

	return result.String()
}

// DisconnectQueue is a rate-limited executor
type DisconnectQueue struct {
	node *Node
	// Throttling rate
	rate time.Duration
	// Call RPC Disconnect for connections
	disconnect *utils.Queue[*Session]
	limit      int
	// Logger with context
	log *slog.Logger
	// Executer stopped status
	isStopped bool
	// Mutex to work with stopped status concurrently
	mu sync.Mutex
}

// NewDisconnectQueue builds new queue with a specified rate (max calls per second)
func NewDisconnectQueue(node *Node, config *DisconnectQueueConfig, l *slog.Logger) *DisconnectQueue {
	rateDuration := time.Millisecond * time.Duration(1000/config.Rate)

	ctx := l.With("context", "disconnector")

	ctx.Debug("calls rate", "rate", rateDuration)

	capacity := config.MaxBacklog / 8

	if capacity < 512 {
		capacity = 512
	}

	return &DisconnectQueue{
		node:       node,
		disconnect: utils.NewQueue[*Session](capacity),
		limit:      config.MaxBacklog,
		rate:       rateDuration,
		log:        ctx,
	}
}

// Run starts queue
func (d *DisconnectQueue) Run() error {
	throttle := time.NewTicker(d.rate)
	defer throttle.Stop()

	for {
		if !d.disconnect.Wait() {
			return nil
		}

		item, ok := d.disconnect.Remove()
		if !ok {
			return nil
		}

		session := item.Data

		d.node.DisconnectNow(session) //nolint:errcheck
		<-throttle.C
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
	sessions := make([]*Session, 0, d.disconnect.Len())

	for {
		item, ok := d.disconnect.Remove()
		if !ok {
			break
		}

		sessions = append(sessions, item.Data)
	}

	d.disconnect.Close()
	d.mu.Unlock()

	actual := 0
	left := len(sessions)

	if left == 0 {
		return nil
	}

	defer func() {
		d.log.Info("disconnected sessions", "num", actual)
	}()

	deadline, ok := ctx.Deadline()

	if ok {
		timeLeft := time.Until(deadline)

		d.log.Info("invoking remaining disconnects", "interval", timeLeft.Seconds(), "num", left)
	} else {
		d.log.Info("invoking remaining disconnects", "num", left)
	}

	for _, session := range sessions {
		select {
		case <-ctx.Done():
			return fmt.Errorf("had no time to invoke Disconnect calls: ~%d", left-actual)
		default:
		}

		d.node.DisconnectNow(session) // nolint:errcheck

		actual++
	}

	return nil
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

	if d.limit > 0 && d.Size() >= d.limit {
		d.node.metrics.CounterIncrement(metricsDisconnectDiscarded)
		return fmt.Errorf("disconnect queue is full, discarding Disconnect")
	}

	d.disconnect.Add(utils.Item[*Session]{Data: s, Size: 1})

	return nil
}

// Size returns the number of enqueued tasks
func (d *DisconnectQueue) Size() int {
	return int(d.disconnect.Size())
}
