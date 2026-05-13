package lib

import "sync"

// Accumulator tracks per-client receive counts. Concurrent-safe via a single
// mutex over the map — workloads are dominated by the network and JSON
// parsing, so finer-grained sharding is not yet worth the complexity.
type Accumulator struct {
	mu     sync.Mutex
	counts map[string]int
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
