// Package stats contains calculation utils for benchmarks
// Based on https://github.com/anycable/websocket-bench/blob/master/benchmark/stat.go
package stats

import (
	"sort"
	"time"
)

// RoundToMS returns the number of milliseconds for the given duration
func RoundToMS(d time.Duration) int64 {
	return int64((d + (500 * time.Microsecond)) / time.Millisecond)
}

// ResAggregate contains duration samples
type ResAggregate struct {
	samples []time.Duration
	sorted  bool
}

type byAsc []time.Duration

func (a byAsc) Len() int           { return len(a) }
func (a byAsc) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byAsc) Less(i, j int) bool { return a[i] < a[j] }

// Add adds a new sample to the aggregate
func (agg *ResAggregate) Add(rtt time.Duration) {
	agg.samples = append(agg.samples, rtt)
	agg.sorted = false
}

// Count returns the number of samples
func (agg *ResAggregate) Count() int {
	return len(agg.samples)
}

// Min returns the min value
func (agg *ResAggregate) Min() time.Duration {
	if agg.Count() == 0 {
		return 0
	}
	agg.sort()
	return agg.samples[0]
}

// Max returns the max value
func (agg *ResAggregate) Max() time.Duration {
	if agg.Count() == 0 {
		return 0
	}
	agg.sort()
	return agg.samples[len(agg.samples)-1]
}

// Percentile returns the p-th percentile
func (agg *ResAggregate) Percentile(p int) time.Duration {
	if p <= 0 {
		panic("p must be greater than 0")
	} else if 100 <= p {
		panic("p must be less 100")
	}

	agg.sort()

	rank := p * len(agg.samples) / 100

	if agg.Count() == 0 {
		return 0
	}

	return agg.samples[rank]
}

func (agg *ResAggregate) sort() {
	if agg.sorted {
		return
	}
	sort.Sort(byAsc(agg.samples))
	agg.sorted = true
}
