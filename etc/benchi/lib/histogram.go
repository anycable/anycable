package lib

import (
	"math"
	"sync"
	"time"
)

// Histogram is a concurrency-safe, bounded-memory latency histogram with
// log-scale buckets (~5% relative precision) from 100µs to ~2h. It is
// sized for "how long did a handshake take" questions under storms where
// millions of samples are possible, so it never stores raw values.
type Histogram struct {
	mu      sync.Mutex
	buckets [histogramBuckets]int64
	count   int64
	max     time.Duration
}

const (
	histogramMin     = 100 * time.Microsecond
	histogramGrowth  = 1.05
	histogramBuckets = 360 // 100µs × 1.05^360 ≈ 4.2h
)

var histogramLogGrowth = math.Log(histogramGrowth)

func bucketFor(d time.Duration) int {
	if d <= histogramMin {
		return 0
	}
	i := int(math.Log(float64(d)/float64(histogramMin))/histogramLogGrowth) + 1
	if i >= histogramBuckets {
		i = histogramBuckets - 1
	}
	return i
}

// bucketUpper returns the upper bound of bucket i (the value Quantile
// reports for samples in that bucket).
func bucketUpper(i int) time.Duration {
	return time.Duration(float64(histogramMin) * math.Pow(histogramGrowth, float64(i)))
}

// Record adds one sample.
func (h *Histogram) Record(d time.Duration) {
	i := bucketFor(d)
	h.mu.Lock()
	h.buckets[i]++
	h.count++
	if d > h.max {
		h.max = d
	}
	h.mu.Unlock()
}

// HistogramSnapshot is an immutable copy of a Histogram.
type HistogramSnapshot struct {
	buckets [histogramBuckets]int64
	Count   int64
	Max     time.Duration
}

// Snapshot copies the current state.
func (h *Histogram) Snapshot() HistogramSnapshot {
	h.mu.Lock()
	defer h.mu.Unlock()
	return HistogramSnapshot{buckets: h.buckets, Count: h.count, Max: h.max}
}

// Swap returns the current state and resets the histogram. Use it to get
// per-interval distributions.
func (h *Histogram) Swap() HistogramSnapshot {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := HistogramSnapshot{buckets: h.buckets, Count: h.count, Max: h.max}
	h.buckets = [histogramBuckets]int64{}
	h.count = 0
	h.max = 0
	return s
}

// Quantile returns the q-quantile (0 < q <= 1), or 0 for an empty snapshot.
// The result is the upper bound of the containing bucket, capped at Max.
func (s HistogramSnapshot) Quantile(q float64) time.Duration {
	if s.Count == 0 {
		return 0
	}
	rank := int64(math.Ceil(q * float64(s.Count)))
	if rank < 1 {
		rank = 1
	}
	var seen int64
	for i, n := range s.buckets {
		seen += n
		if seen >= rank {
			v := bucketUpper(i)
			if v > s.Max {
				v = s.Max
			}
			return v
		}
	}
	return s.Max
}

// Sub returns the distribution of samples recorded between prev and s
// (prev must be an earlier snapshot of the same histogram). Max is
// approximated by the upper bound of the highest non-empty bucket.
func (s HistogramSnapshot) Sub(prev HistogramSnapshot) HistogramSnapshot {
	out := HistogramSnapshot{Count: s.Count - prev.Count}
	for i := range s.buckets {
		out.buckets[i] = s.buckets[i] - prev.buckets[i]
		if out.buckets[i] > 0 {
			out.Max = bucketUpper(i)
		}
	}
	if out.Max > s.Max {
		out.Max = s.Max
	}
	return out
}
