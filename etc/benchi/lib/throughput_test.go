package lib_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/anycable/anycable-go/etc/benchi/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReceiveSampler_CapturesGrowingCounter(t *testing.T) {
	var n atomic.Int64
	s := lib.NewReceiveSampler(20*time.Millisecond, n.Load)
	s.Start()
	for i := 1; i <= 5; i++ {
		time.Sleep(25 * time.Millisecond)
		n.Store(int64(i * 10))
	}
	s.Stop()

	samples := s.Samples()
	require.GreaterOrEqual(t, len(samples), 5, "should have at least 5 samples in ~125ms with 20ms tick")
	assert.EqualValues(t, 0, samples[0].N, "first sample taken at Start, before any increments")
	assert.EqualValues(t, 50, samples[len(samples)-1].N, "final sample should reflect last counter value")
}

func TestReceiveSampler_DoubleStopIsSafe(t *testing.T) {
	var n atomic.Int64
	s := lib.NewReceiveSampler(20*time.Millisecond, n.Load)
	s.Start()
	s.Stop()
	assert.NotPanics(t, func() { s.Stop() })
}

func TestComputeWindowStats_ConstantRate(t *testing.T) {
	// 1000 msg/s constant for 3 seconds, sampled every 100ms.
	start := time.Unix(0, 0)
	var samples []lib.ReceiveSample
	for i := 0; i <= 30; i++ {
		samples = append(samples, lib.ReceiveSample{
			T: start.Add(time.Duration(i) * 100 * time.Millisecond),
			N: int64(i * 100),
		})
	}
	stats := lib.ComputeWindowStats(samples, time.Second)
	require.True(t, stats.Valid)
	assert.InDelta(t, 1000.0, stats.First, 1.0)
	assert.InDelta(t, 1000.0, stats.Last, 1.0)
	assert.InDelta(t, 1000.0, stats.Min, 1.0)
	assert.InDelta(t, 1000.0, stats.Max, 1.0)
}

func TestComputeWindowStats_BurstThenIdle(t *testing.T) {
	// 0..1s: 2000 msg/s; 1s..3s: 0 msg/s.
	start := time.Unix(0, 0)
	var samples []lib.ReceiveSample
	for i := 0; i <= 10; i++ {
		samples = append(samples, lib.ReceiveSample{
			T: start.Add(time.Duration(i) * 100 * time.Millisecond),
			N: int64(i * 200),
		})
	}
	for i := 11; i <= 30; i++ {
		samples = append(samples, lib.ReceiveSample{
			T: start.Add(time.Duration(i) * 100 * time.Millisecond),
			N: 2000,
		})
	}
	stats := lib.ComputeWindowStats(samples, time.Second)
	require.True(t, stats.Valid)
	assert.InDelta(t, 2000.0, stats.Max, 10.0, "max should catch the burst window")
	assert.InDelta(t, 0.0, stats.Last, 1.0, "last window is idle")
	assert.InDelta(t, 0.0, stats.Min, 10.0, "min should catch the idle window")
}

func TestComputeWindowStats_TooShort(t *testing.T) {
	start := time.Unix(0, 0)
	samples := []lib.ReceiveSample{
		{T: start, N: 0},
		{T: start.Add(100 * time.Millisecond), N: 10},
	}
	stats := lib.ComputeWindowStats(samples, time.Second)
	assert.False(t, stats.Valid, "sampling shorter than the window must report Valid=false")
}

func TestComputeWindowStats_EmptyOrInvalid(t *testing.T) {
	assert.False(t, lib.ComputeWindowStats(nil, time.Second).Valid)
	assert.False(t, lib.ComputeWindowStats([]lib.ReceiveSample{{T: time.Now(), N: 0}}, time.Second).Valid)
	assert.False(t, lib.ComputeWindowStats(
		[]lib.ReceiveSample{{T: time.Now(), N: 0}, {T: time.Now(), N: 0}},
		0,
	).Valid)
}
