package lib

import (
	"sync"
	"sync/atomic"
)

// Accumulator tracks per-client receive counts. Concurrent-safe via a single
// mutex over the map — workloads are dominated by the network and JSON
// parsing, so finer-grained sharding is not yet worth the complexity.
//
// A separate atomic running total is maintained alongside the map so
// TotalReceived can be sampled cheaply (no mutex, no map walk) from the
// throughput sampler hot path.
type Accumulator struct {
	mu     sync.Mutex
	counts map[string]int
	total  atomic.Int64
}

// NewAccumulator returns an empty Accumulator ready for concurrent Bump.
func NewAccumulator() *Accumulator {
	return &Accumulator{counts: make(map[string]int)}
}

// Bump increments the count for clientID by one.
func (a *Accumulator) Bump(clientID string) {
	a.mu.Lock()
	a.counts[clientID]++
	a.mu.Unlock()
	a.total.Add(1)
}

// TotalReceived returns the running total of Bump calls. Safe to call from
// any goroutine without taking the map mutex.
func (a *Accumulator) TotalReceived() int64 {
	return a.total.Load()
}

// Snapshot returns a copy of the current counts. Callers may mutate the
// returned map freely without affecting future Snapshot results.
func (a *Accumulator) Snapshot() map[string]int {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make(map[string]int, len(a.counts))
	for k, v := range a.counts {
		out[k] = v
	}
	return out
}
