package lib

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// publishTask carries one queued broadcast from Publish to a worker.
type publishTask struct {
	stream string
	data   string
}

// Publisher is a bounded async HTTP broadcast client. Publish does not block on
// HTTP latency: tasks enqueue into a buffered channel (capacity = maxInflight)
// served by a small worker pool. When the queue is full, Publish increments a
// dropped counter and returns — "dropped" means the scheduler outran the HTTP
// pipeline, not that the server rejected the call. Non-2xx server responses
// are still counted as issued (the request left the building); the summary
// distinguishes the two failure modes.
type Publisher struct {
	url    string
	client *http.Client
	tasks  chan publishTask

	issued  atomic.Int64
	dropped atomic.Int64

	targetMu sync.Mutex
	targets  map[string]int

	mu     sync.RWMutex
	closed bool

	wg sync.WaitGroup
}

// NewPublisher builds a Publisher targeting serverURL with a queue of
// maxInflight tasks served by runtime.NumCPU() workers sharing a tuned
// http.Transport for connection reuse.
func NewPublisher(serverURL string, maxInflight int) *Publisher {
	if maxInflight < 1 {
		maxInflight = 1
	}
	transport := &http.Transport{
		MaxIdleConns:        maxInflight * 2,
		MaxIdleConnsPerHost: maxInflight * 2,
		IdleConnTimeout:     90 * time.Second,
	}
	p := &Publisher{
		url:     serverURL,
		client:  &http.Client{Transport: transport, Timeout: 30 * time.Second},
		tasks:   make(chan publishTask, maxInflight),
		targets: make(map[string]int),
	}
	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	p.wg.Add(workers)
	for range workers {
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
	for task := range p.tasks {
		p.dispatch(task)
	}
}

func (p *Publisher) dispatch(task publishTask) {
	body, err := json.Marshal(struct {
		Stream string `json:"stream"`
		Data   string `json:"data"`
	}{Stream: task.stream, Data: task.data})
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
}
