package lib

import (
	"sync"
	"time"
)

// ReceiveSampler periodically polls a monotonic counter and records the
// (timestamp, value) pair. After Stop, the collected series can be reduced
// to sliding-window rate statistics via WindowStats.
//
// The sampler is single-purpose: build the receive-rate timeline for a
// benchmark run. It is not a generic time-series store — samples are kept
// in memory in their entirety, sized by (run duration / interval). For a
// 10-second `-d` and 100ms interval at a 250-second wallclock run, that's
// ~2,500 samples — trivial.
type ReceiveSampler struct {
	interval time.Duration
	source   func() int64

	mu       sync.Mutex
	samples  []ReceiveSample
	stopCh   chan struct{}
	doneCh   chan struct{}
	startTime time.Time
	stopped  bool
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

// WindowStats summarizes the receive rate computed over sliding windows of
// a fixed size. Rate is messages-per-second. Zero values mean the sampling
// duration was shorter than the window size and no full window could be
// evaluated — caller should detect this via Valid.
type WindowStats struct {
	First float64
	Last  float64
	Min   float64
	Max   float64
	Valid bool
}

// ComputeWindowStats reduces a sample timeline to first/last/min/max rates
// over sliding windows of size `window`. For each sample i, the function
// finds the latest sample j such that samples[j].T <= samples[i].T - window,
// and computes the rate (samples[i].N - samples[j].N) / dt where dt is the
// real interval between the two samples (≈ window, off by at most one
// sampling interval). If no full window exists (total sampling duration
// shorter than `window`), Valid is false.
//
// Implementation note: two-pointer scan, O(n). j only ever advances, so
// the inner loop is amortized O(1) per i.
func ComputeWindowStats(samples []ReceiveSample, window time.Duration) WindowStats {
	if len(samples) < 2 || window <= 0 {
		return WindowStats{}
	}
	var stats WindowStats
	j := 0
	for i := 1; i < len(samples); i++ {
		// Advance j to the latest sample whose T is still <= samples[i].T - window.
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
		rate := float64(samples[i].N-samples[j].N) / dt
		if !stats.Valid {
			stats.First = rate
			stats.Min = rate
			stats.Max = rate
			stats.Valid = true
		} else {
			if rate < stats.Min {
				stats.Min = rate
			}
			if rate > stats.Max {
				stats.Max = rate
			}
		}
		stats.Last = rate
	}
	return stats
}
