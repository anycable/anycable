package lib

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"runtime"
	"strconv"
	"sync"
)

// PoolConfig describes the shape of a ClientPool build. Most fields map
// 1:1 to the scenario CLI flags: C clients each subscribe to a random
// S-sized subset of the streams 1..BigS. Setup tolerates up to Tolerance
// individual client failures before BuildPool returns an error.
type PoolConfig struct {
	// ServerURL is the WebSocket endpoint each client dials.
	ServerURL string

	// C is the number of clients to spin up.
	C int

	// S is the per-client subscription count (subset size).
	S int

	// BigS is the universe of stream names, [1..BigS]. Streams are named by
	// their integer position rendered as a decimal string.
	BigS int

	// Tolerance is the maximum number of setup failures BuildPool will
	// accept before returning an error. Failures over the limit cause
	// BuildPool to close any clients it managed to bring up and return.
	Tolerance int

	// Seed seeds the RNG for subset selection — same seed, same subsets.
	Seed uint64

	// Accumulator, when non-nil, receives a Bump(clientID) call for every
	// message a pool client receives, until Close.
	Accumulator *Accumulator

	// Streams overrides the random subset generation. Length must equal C
	// when non-nil. Used primarily by tests to inject specific subscription
	// patterns (including ones designed to fail).
	Streams [][]string
}

// poolEntry binds a client to its assigned ID and stream subset so the drain
// goroutine can attribute messages and the test can introspect subscriptions.
type poolEntry struct {
	id      string
	client  *Client
	streams []string
}

// ClientPool owns a set of connected clients and their drain goroutines. The
// pool is built ready-to-receive: any messages dispatched by the server
// after BuildPool returns flow through Receive into the configured
// Accumulator.
type ClientPool struct {
	entries []poolEntry
	acc     *Accumulator
	drainWG sync.WaitGroup

	mu     sync.Mutex
	closed bool
}

// BuildPool builds a pool of cfg.C clients connected to cfg.ServerURL,
// subscribed to randomly-chosen S-sized stream subsets, and ready to
// receive. Setup runs with bounded parallelism — too many concurrent dials
// would overwhelm the embedded server's accept loop and produce false
// timeouts.
//
// On success the returned pool has its drain goroutines already running;
// callers must Close it before the underlying server shuts down to avoid
// leaking goroutines.
func BuildPool(cfg PoolConfig) (*ClientPool, error) {
	if cfg.C <= 0 {
		return nil, errors.New("PoolConfig.C must be positive")
	}
	if cfg.S < 0 {
		return nil, errors.New("PoolConfig.S must be non-negative")
	}
	if cfg.Streams != nil && len(cfg.Streams) != cfg.C {
		return nil, fmt.Errorf("PoolConfig.Streams has length %d, want %d", len(cfg.Streams), cfg.C)
	}

	subsets := cfg.Streams
	if subsets == nil {
		subsets = BuildStreamSubsets(cfg.Seed, cfg.C, cfg.S, cfg.BigS)
	}

	concurrency := runtime.NumCPU() * 8
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > cfg.C {
		concurrency = cfg.C
	}
	sem := make(chan struct{}, concurrency)

	type buildResult struct {
		idx     int
		entry   poolEntry
		err     error
		failure string
	}
	results := make([]buildResult, cfg.C)

	var wg sync.WaitGroup
	wg.Add(cfg.C)
	for i := range cfg.C {
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			id := strconv.Itoa(idx)
			streams := subsets[idx]

			cl, err := BuildClient(cfg.ServerURL)
			if err != nil {
				results[idx] = buildResult{idx: idx, err: err, failure: fmt.Sprintf("client %s: build: %v", id, err)}
				return
			}
			if err := cl.Connect(context.Background()); err != nil {
				cl.Close()
				results[idx] = buildResult{idx: idx, err: err, failure: fmt.Sprintf("client %s: connect: %v", id, err)}
				return
			}
			if err := cl.Subscribe(streams); err != nil {
				cl.Close()
				results[idx] = buildResult{idx: idx, err: err, failure: fmt.Sprintf("client %s: subscribe: %v", id, err)}
				return
			}

			results[idx] = buildResult{
				idx:   idx,
				entry: poolEntry{id: id, client: cl, streams: streams},
			}
		}(i)
	}
	wg.Wait()

	var failures []string
	entries := make([]poolEntry, 0, cfg.C)
	for _, r := range results {
		if r.err != nil {
			failures = append(failures, r.failure)
			continue
		}
		entries = append(entries, r.entry)
	}

	if len(failures) > cfg.Tolerance {
		for _, e := range entries {
			e.client.Close()
		}
		return nil, fmt.Errorf("pool setup failed: %d failure(s) exceed tolerance %d: %v", len(failures), cfg.Tolerance, failures)
	}

	pool := &ClientPool{entries: entries, acc: cfg.Accumulator}
	for _, e := range entries {
		pool.drainWG.Add(1)
		go pool.drain(e)
	}
	return pool, nil
}

func (p *ClientPool) drain(e poolEntry) {
	defer p.drainWG.Done()
	for {
		_, ok := e.client.Receive()
		if !ok {
			return
		}
		if p.acc != nil {
			p.acc.Bump(e.id)
		}
	}
}

// Each runs fn for every client in the pool, in pool order.
func (p *ClientPool) Each(fn func(*Client)) {
	for _, e := range p.entries {
		fn(e.client)
	}
}

// Size returns the number of successfully built clients in the pool.
func (p *ClientPool) Size() int { return len(p.entries) }

// Close stops every client in the pool and waits for the drain goroutines
// to finish. Safe to call more than once.
func (p *ClientPool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()

	for _, e := range p.entries {
		e.client.Close()
	}
	p.drainWG.Wait()
}

// Streams returns each client's assigned stream subset, in pool order.
// Primarily used by tests to verify the random-subset distribution.
func (p *ClientPool) Streams() [][]string {
	out := make([][]string, len(p.entries))
	for i, e := range p.entries {
		out[i] = append([]string(nil), e.streams...)
	}
	return out
}

// BuildStreamSubsets generates C uniformly-random S-sized subsets of the
// stream universe 1..BigS, seeded for reproducibility. Exported so tests
// can audit the distribution without spinning up real clients.
func BuildStreamSubsets(seed uint64, c, s, bigS int) [][]string {
	rng := rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))
	out := make([][]string, c)
	for i := range c {
		out[i] = pickSubset(rng, s, bigS)
	}
	return out
}

func pickSubset(rng *rand.Rand, s, bigS int) []string {
	if bigS <= 0 || s <= 0 {
		return nil
	}
	if s > bigS {
		s = bigS
	}
	universe := make([]int, bigS)
	for i := range bigS {
		universe[i] = i + 1
	}
	rng.Shuffle(bigS, func(i, j int) { universe[i], universe[j] = universe[j], universe[i] })
	out := make([]string, s)
	for i := range s {
		out[i] = strconv.Itoa(universe[i])
	}
	return out
}
