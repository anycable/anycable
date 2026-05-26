package lib

import (
	"sort"
	"sync"
	"time"
)

// ReceiveSampler periodically polls a monotonic counter and records the
// (timestamp, value) pair. After Stop, the collected series can be reduced
// to sliding-window rate statistics via WindowStats.
//
// The sampler is single-purpose: build a rate timeline for a benchmark run
// from a monotonic counter (received messages, completed POSTs, etc.). It
// is not a generic time-series store — samples are kept in memory in their
// entirety, sized by (run duration / interval). For a 10-second `-d` and
// 100ms interval at a 250-second wallclock run, that's ~2,500 samples —
// trivial.
type ReceiveSampler struct {
	interval time.Duration
	source   func() int64

	mu        sync.Mutex
	samples   []ReceiveSample
	stopCh    chan struct{}
	doneCh    chan struct{}
	startTime time.Time
	stopped   bool
}

// ReceiveSample is one (timestamp, count) pair from the sampler.
type ReceiveSample struct {
	T time.Time
	N int64
}

// NewReceiveSampler builds a sampler that polls source every interval. The
// sampler does not start until Start is called.
func NewReceiveSampler(interval time.Duration, source func() int64) *ReceiveSampler {
	return &ReceiveSampler{
		interval: interval,
		source:   source,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start kicks off the background sampling goroutine. The first sample is
// taken synchronously so the timeline has a known origin even if Stop is
// called before the first tick.
func (s *ReceiveSampler) Start() {
	s.mu.Lock()
	s.startTime = time.Now()
	s.samples = append(s.samples, ReceiveSample{T: s.startTime, N: s.source()})
	s.mu.Unlock()

	go func() {
		defer close(s.doneCh)
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			select {
			case now := <-t.C:
				s.mu.Lock()
				s.samples = append(s.samples, ReceiveSample{T: now, N: s.source()})
				s.mu.Unlock()
			case <-s.stopCh:
				// Take one final sample so the last window aligns with the
				// caller's notion of "end of run" rather than the previous
				// tick.
				s.mu.Lock()
				s.samples = append(s.samples, ReceiveSample{T: time.Now(), N: s.source()})
				s.mu.Unlock()
				return
			}
		}
	}()
}

// Stop terminates the sampling goroutine and waits for it to exit. Safe to
// call more than once.
func (s *ReceiveSampler) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		<-s.doneCh
		return
	}
	s.stopped = true
	close(s.stopCh)
	s.mu.Unlock()
	<-s.doneCh
}

// Samples returns a copy of the collected samples. Safe to call before or
// after Stop; the returned slice is decoupled from the sampler's internal
// storage.
func (s *ReceiveSampler) Samples() []ReceiveSample {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ReceiveSample, len(s.samples))
	copy(out, s.samples)
	return out
}

// Overall returns the average rate across the full sample timeline:
// (last.N - first.N) / (last.T - first.T). Zero if fewer than two samples
// or zero duration.
func Overall(samples []ReceiveSample) float64 {
	if len(samples) < 2 {
		return 0
	}
	first := samples[0]
	last := samples[len(samples)-1]
	dt := last.T.Sub(first.T).Seconds()
	if dt <= 0 {
		return 0
	}
	return float64(last.N-first.N) / dt
}

// WindowStats summarizes the rate computed over sliding windows of a fixed
// size. Rate is messages-per-second. Zero values mean the sampling duration
// was shorter than the window size and no full window could be evaluated —
// caller should detect this via Valid.
//
// P50/P95 are percentiles across the population of window-rate samples and
// describe distribution shape: P50 is the typical sustained rate, P95 the
// near-peak. Together with Max/Min they paint the headroom and stall story
// without requiring the caller to plot the full timeline.
type WindowStats struct {
	Max   float64
	Min   float64
	P50   float64
	P95   float64
	Valid bool
}

// ComputeWindowStats reduces a sample timeline to max/min/p50/p95 rates over
// sliding windows of size `window`. For each sample i, the function finds
// the latest sample j such that samples[j].T <= samples[i].T - window, and
// computes the rate (samples[i].N - samples[j].N) / dt where dt is the real
// interval between the two samples (≈ window, off by at most one sampling
// interval). If no full window exists (total sampling duration shorter than
// `window`), Valid is false.
//
// Implementation note: two-pointer scan to collect rates is O(n); then one
// sort for percentiles. j only ever advances, so the rate-collection loop
// is amortized O(1) per i.
func ComputeWindowStats(samples []ReceiveSample, window time.Duration) WindowStats {
	if len(samples) < 2 || window <= 0 {
		return WindowStats{}
	}
	var rates []float64
	j := 0
	for i := 1; i < len(samples); i++ {
		for j+1 < i && samples[i].T.Sub(samples[j+1].T) >= window {
			j++
		}
		if samples[i].T.Sub(samples[j].T) < window {
			// Haven't accumulated a full window yet.
			continue
		}
		dt := samples[i].T.Sub(samples[j].T).Seconds()
		if dt <= 0 {
			continue
		}
		rates = append(rates, float64(samples[i].N-samples[j].N)/dt)
	}
	if len(rates) == 0 {
		return WindowStats{}
	}

	minR, maxR := rates[0], rates[0]
	for _, r := range rates[1:] {
		if r < minR {
			minR = r
		}
		if r > maxR {
			maxR = r
		}
	}

	sorted := append([]float64(nil), rates...)
	sort.Float64s(sorted)

	return WindowStats{
		Max:   maxR,
		Min:   minR,
		P50:   percentile(sorted, 0.50),
		P95:   percentile(sorted, 0.95),
		Valid: true,
	}
}

// percentile returns the p-th percentile (p in [0, 1]) of a sorted slice
// using linear interpolation between adjacent values. Matches numpy's
// default behavior so output is comparable to ad-hoc analysis scripts.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := p * float64(len(sorted)-1)
	lo := int(rank)
	if lo >= len(sorted)-1 {
		return sorted[len(sorted)-1]
	}
	frac := rank - float64(lo)
	return sorted[lo] + frac*(sorted[lo+1]-sorted[lo])
}
