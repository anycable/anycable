package lib

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// publishTask carries one queued broadcast from Publish to a worker.
type publishTask struct {
	stream string
	data   string
}

// Publisher is a bounded async HTTP broadcast client. Publish does not block
// on HTTP latency: tasks enqueue into a buffered channel (capacity =
// maxInflight) served by a worker pool. When the queue is full, Publish
// increments a dropped counter and returns — "dropped" means the scheduler
// outran the HTTP pipeline, not that the server rejected the call.
//
// Workers coalesce queued tasks into batches: after a blocking read of the
// first task, a worker drains up to batchSize-1 more tasks non-blockingly
// and POSTs them as a single JSON array (the broadcast endpoint accepts
// either an object or an array of objects). At low load batches collapse to
// size 1 — no added latency, no behavior change vs the pre-batching
// publisher. Under backpressure, batches grow up to batchSize, amortizing
// HTTP overhead.
//
// Three counters are exposed:
//   - IssuedCount: tasks successfully enqueued (deterministic for the
//     tick-grid scheduler; ≈ "what we tried to publish").
//   - DroppedCount: enqueue attempts rejected because the queue was full
//     (or because Publish was called after Close).
//   - CompletedCount: broadcasts whose batch POST returned 2xx. This is the
//     number that matters for "actual publish rate". Non-2xx and transport
//     errors do not increment CompletedCount; they remain in IssuedCount.
type Publisher struct {
	url    string
	client *http.Client
	tasks  chan publishTask

	workers   int
	batchSize int

	issued    atomic.Int64
	dropped   atomic.Int64
	completed atomic.Int64

	targetMu sync.Mutex
	targets  map[string]int

	mu     sync.RWMutex
	closed bool

	wg sync.WaitGroup
}

// Option configures a Publisher at construction time.
type Option func(*publisherOpts)

type publisherOpts struct {
	workers   int
	batchSize int
}

// WithWorkers sets the number of HTTP worker goroutines. Workers are I/O
// bound — they spend most of their time in http.Client.Do — so a count
// well above NumCPU is correct. Default in NewPublisher is 64, sized for
// the embedded-server scenario where each POST triggers synchronous
// fan-out across every subscribed client (high concurrency multiplies
// server-side work and can exhaust the Go OS-thread limit). For external
// broadcasters where each POST is cheap (NATS, Redis, an edge service)
// values up to ~1024 are reasonable.
func WithWorkers(n int) Option {
	return func(o *publisherOpts) {
		if n > 0 {
			o.workers = n
		}
	}
}

// WithBatchSize sets the maximum number of broadcasts a single worker will
// pack into one HTTP POST. The first task in a batch is a blocking read; up
// to batchSize-1 additional tasks are non-blockingly drained from the queue
// before dispatch. batchSize=1 disables batching (one POST per broadcast).
// Default in NewPublisher is 64.
func WithBatchSize(n int) Option {
	return func(o *publisherOpts) {
		if n > 0 {
			o.batchSize = n
		}
	}
}

// NewPublisher builds a Publisher targeting serverURL with a queue of
// maxInflight tasks. Defaults: 64 workers, batch size 64. Override with
// WithWorkers / WithBatchSize.
func NewPublisher(serverURL string, maxInflight int, opts ...Option) *Publisher {
	if maxInflight < 1 {
		maxInflight = 1
	}
	cfg := publisherOpts{workers: 64, batchSize: 64}
	for _, opt := range opts {
		opt(&cfg)
	}
	// HTTP idle connection pool sized to the worker count so workers don't
	// thrash setting up TCP connections under steady load.
	idleConns := cfg.workers
	if idleConns < maxInflight {
		idleConns = maxInflight
	}
	transport := &http.Transport{
		MaxIdleConns:        idleConns,
		MaxIdleConnsPerHost: idleConns,
		IdleConnTimeout:     90 * time.Second,
	}
	p := &Publisher{
		url:       serverURL,
		client:    &http.Client{Transport: transport, Timeout: 30 * time.Second},
		tasks:     make(chan publishTask, maxInflight),
		workers:   cfg.workers,
		batchSize: cfg.batchSize,
		targets:   make(map[string]int),
	}
	p.wg.Add(cfg.workers)
	for range cfg.workers {
		go p.worker()
	}
	return p
}

// Publish non-blockingly enqueues a broadcast. On success, increments the
// issued counter and records the stream in the target-counts snapshot. On a
// full queue or after Close, increments the dropped counter and returns.
func (p *Publisher) Publish(stream, data string) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		p.dropped.Add(1)
		return
	}
	select {
	case p.tasks <- publishTask{stream: stream, data: data}:
		p.issued.Add(1)
		p.targetMu.Lock()
		p.targets[stream]++
		p.targetMu.Unlock()
	default:
		p.dropped.Add(1)
	}
}

// IssuedCount returns the number of successfully enqueued tasks.
func (p *Publisher) IssuedCount() int64 { return p.issued.Load() }

// DroppedCount returns the number of tasks dropped because the queue was full
// (or because Publish was called after Close).
func (p *Publisher) DroppedCount() int64 { return p.dropped.Load() }

// CompletedCount returns the number of broadcasts whose batch POST returned
// a 2xx response. This is the count of publishes the server actually
// accepted — the real publish rate divides this by wall-clock time.
func (p *Publisher) CompletedCount() int64 { return p.completed.Load() }

// TargetCounts returns a snapshot of per-stream issued counts. Safe to call
// before or after Close; the caller owns the returned map.
func (p *Publisher) TargetCounts() map[string]int {
	p.targetMu.Lock()
	defer p.targetMu.Unlock()
	out := make(map[string]int, len(p.targets))
	for k, v := range p.targets {
		out[k] = v
	}
	return out
}

// Close stops accepting new tasks, lets workers finish what is already
// queued, and waits for them to exit. Safe to call more than once.
func (p *Publisher) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	close(p.tasks)
	p.mu.Unlock()
	p.wg.Wait()
}

func (p *Publisher) worker() {
	defer p.wg.Done()
	batch := make([]publishTask, 0, p.batchSize)
	for first := range p.tasks {
		batch = append(batch[:0], first)
	drain:
		for len(batch) < p.batchSize {
			select {
			case t, ok := <-p.tasks:
				if !ok {
					break drain
				}
				batch = append(batch, t)
			default:
				break drain
			}
		}
		p.dispatch(batch)
	}
}

// broadcastMessage matches AnyCable's HTTP broadcast payload shape. The
// server accepts either a single object or a JSON array of objects
// (node/node.go HandleBroadcast handles both); we always send an array so
// the same code path works for batchSize=1 and batchSize=N.
type broadcastMessage struct {
	Stream string `json:"stream"`
	Data   string `json:"data"`
}

func (p *Publisher) dispatch(batch []publishTask) {
	payload := make([]broadcastMessage, len(batch))
	for i, t := range batch {
		payload[i] = broadcastMessage{Stream: t.stream, Data: t.data}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, p.url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		p.completed.Add(int64(len(batch)))
	}
}
