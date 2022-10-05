package rpc

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSmartBarrierGrow(t *testing.T) {
	config := buildConfig(5)
	barrier := NewSmartBarrier(config, slog.Default())
	defer barrier.close()

	assert.Equal(t, 5, barrier.Capacity())

	for i := 0; i < 15; i++ {
		go barrier.Acquire()
	}

	// Wait go routines to call acquire
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 5, barrier.BusyCount())

	barrier.tick()
	assert.Equal(t, 5, barrier.Capacity())

	go barrier.Acquire()

	// Wait go routines to call acquire
	time.Sleep(100 * time.Millisecond)

	barrier.tick()
	assert.Equal(t, 7, barrier.Capacity())

	assert.Equal(t, 7, barrier.BusyCount())
}

func TestSmartBarrierShrink(t *testing.T) {
	config := buildConfig(5)
	barrier := NewSmartBarrier(config, slog.Default())
	defer barrier.close()

	assert.Equal(t, 5, barrier.Capacity())

	// Simulate 20 calls
	for i := 0; i < 20; i++ {
		barrier.Acquire()
		barrier.Release()
	}

	// Only 1 was exhausted—not enough to shrink
	barrier.Exhausted()

	barrier.tick()
	assert.Equal(t, 5, barrier.Capacity())

	for i := 0; i < 20; i++ {
		barrier.Acquire()
		barrier.Release()
	}

	// 3 out of 20 is enough to trigger shrink
	barrier.Exhausted()
	barrier.Exhausted()
	barrier.Exhausted()

	barrier.tick()
	assert.Equal(t, 4, barrier.Capacity())

	for i := 0; i < 5; i++ {
		go barrier.Acquire()
	}

	// Wait go routines to call acquire
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 4, barrier.BusyCount())
}

func TestSmartBarrierGrowRevert(t *testing.T) {
	config := buildConfig(2)
	barrier := NewSmartBarrier(config, slog.Default())
	defer barrier.close()

	for i := 0; i < 7; i++ {
		go barrier.Acquire()
	}

	// Wait go routines to call acquire
	time.Sleep(100 * time.Millisecond)

	barrier.tick()
	assert.Equal(t, 3, barrier.Capacity())

	for i := 0; i < 7; i++ {
		barrier.Release()
	}

	// During the next interval, we see exhausted increasing
	for i := 0; i < 10; i++ {
		barrier.Acquire()
		barrier.Release()
	}

	barrier.Exhausted()
	barrier.Exhausted()

	barrier.tick()
	// We do not shrink immediately, only on the next tick
	assert.Equal(t, 3, barrier.Capacity())

	// During the next interval, we see exhausted again
	for i := 0; i < 10; i++ {
		barrier.Acquire()
		barrier.Release()
	}

	barrier.Exhausted()
	barrier.Exhausted()

	barrier.tick()
	assert.Equal(t, 2, barrier.Capacity())
}

func TestSmartBarrierShrinkRevert(t *testing.T) {
	config := buildConfig(3)
	barrier := NewSmartBarrier(config, slog.Default())
	defer barrier.close()

	for i := 0; i < 10; i++ {
		barrier.Acquire()
		barrier.Release()
	}

	barrier.Exhausted()
	barrier.Exhausted()

	barrier.tick()
	assert.Equal(t, 2, barrier.Capacity())

	for i := 0; i < 7; i++ {
		go barrier.Acquire()
	}

	// Wait go routines to call acquire
	time.Sleep(100 * time.Millisecond)

	barrier.tick()
	// We do not grow immediately, only on the next tick
	assert.Equal(t, 2, barrier.Capacity())

	for i := 0; i < 7; i++ {
		barrier.Release()
	}

	for i := 0; i < 7; i++ {
		go barrier.Acquire()
	}

	// Wait go routines to call acquire
	time.Sleep(100 * time.Millisecond)

	barrier.tick()
	assert.Equal(t, 3, barrier.Capacity())
}

// Example real stats:
//
//	rpc_call_total=1716 rpc_capacity_num=28 rpc_pending_num=117 rpc_retries_total=266
//
// Should shrink, since the number of retries is bigger than the number of pending calls
// — it's better to wait than to fail.
func TestSmartBarrierShrinkWhenManyPending(t *testing.T) {
	config := buildConfig(5)
	barrier := NewSmartBarrier(config, slog.Default())
	defer barrier.close()

	assert.Equal(t, 5, barrier.Capacity())

	// 26 acquire -> 16 pending > 3 x cap
	for i := 0; i < 26; i++ {
		go barrier.Acquire()
	}

	for i := 0; i < 5; i++ {
		barrier.Release()
	}

	// Wait go routines to call acquire
	time.Sleep(100 * time.Millisecond)

	// 6 times exhausted
	for i := 0; i < 6; i++ {
		barrier.Exhausted()
	}

	go barrier.tick()

	for i := 0; i < 10; i++ {
		barrier.Release()
	}

	// Wait go routines
	time.Sleep(100 * time.Millisecond)

	// Should shrink
	assert.Equal(t, 4, barrier.Capacity())
}

func buildConfig(capacity int) SmartBarrierConfig {
	return SmartBarrierConfig{
		InitialConcurrency: capacity,
		MinConcurrency:     1,
		MaxConcurrency:     10,
		PendingFactor:      3,
		ExhaustedFactor:    0.1,
		GrowthFactor:       1.4,
		// Large enough value to not interfere with tests
		TickInterval: 600,
	}
}
