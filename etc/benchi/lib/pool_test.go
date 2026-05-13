package lib_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/anycable/anycable-go/etc/benchi/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPool_AllSubscribeOk(t *testing.T) {
	srv, err := lib.BuildServer(lib.ServerConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	start := time.Now()
	pool, err := lib.BuildPool(lib.PoolConfig{
		ServerURL: srv.WebSocketURL(),
		C:         10,
		S:         3,
		BigS:      20,
		Tolerance: 0,
		Seed:      42,
	})
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	assert.Less(t, time.Since(start), time.Second, "c=10 happy-path setup must complete in <1s")
	assert.Equal(t, 10, pool.Size())

	for i, streams := range pool.Streams() {
		assert.Len(t, streams, 3, "client %d has wrong subscription count", i)
		seen := make(map[string]bool, len(streams))
		for _, s := range streams {
			n, err := strconv.Atoi(s)
			require.NoError(t, err, "stream name must be a 1..S integer")
			assert.GreaterOrEqual(t, n, 1)
			assert.LessOrEqual(t, n, 20)
			assert.False(t, seen[s], "client %d has duplicate %q in subset", i, s)
			seen[s] = true
		}
	}
}

func TestBuildPool_AccumulatorIntegration(t *testing.T) {
	srv, err := lib.BuildServer(lib.ServerConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	// Hand-pick subsets so we know exactly which 3 of 10 clients subscribe
	// to "target". Clients 0, 4, 7 are subscribed to "target"; everyone else
	// subscribes to a unique singleton stream.
	target := "target"
	subscribers := map[string]bool{"0": true, "4": true, "7": true}
	streams := make([][]string, 10)
	for i := range streams {
		id := strconv.Itoa(i)
		if subscribers[id] {
			streams[i] = []string{target}
		} else {
			streams[i] = []string{"solo-" + id}
		}
	}

	acc := lib.NewAccumulator()
	pool, err := lib.BuildPool(lib.PoolConfig{
		ServerURL:   srv.WebSocketURL(),
		C:           10,
		S:           1,
		BigS:        1,
		Streams:     streams,
		Accumulator: acc,
	})
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	pub := lib.NewPublisher(srv.BroadcastURL(), 16)
	t.Cleanup(pub.Close)
	for range 5 {
		pub.Publish(target, "ping")
	}

	require.Eventually(t, func() bool {
		snap := acc.Snapshot()
		for id := range subscribers {
			if snap[id] != 5 {
				return false
			}
		}
		return true
	}, 3*time.Second, 25*time.Millisecond, "subscribers should each accumulate 5 messages")

	snap := acc.Snapshot()
	for id := range subscribers {
		assert.Equal(t, 5, snap[id], "client %s should receive 5", id)
	}
	for i := range 10 {
		id := strconv.Itoa(i)
		if subscribers[id] {
			continue
		}
		assert.Equal(t, 0, snap[id], "client %s should receive 0", id)
	}
}

func TestBuildPool_StreamSubsetUniformity_Smoke(t *testing.T) {
	const (
		c    = 1000
		s    = 10
		bigS = 100
	)
	subsets := lib.BuildStreamSubsets(0xDECAFC0FFEE, c, s, bigS)
	require.Len(t, subsets, c)

	streamHits := make(map[string]int, bigS)
	for _, sub := range subsets {
		require.Len(t, sub, s)
		seen := make(map[string]bool, s)
		for _, name := range sub {
			assert.False(t, seen[name], "duplicate %q within a single subset", name)
			seen[name] = true
			streamHits[name]++
		}
	}

	// Expected per-stream count is c*s/bigS = 100. The per-stream count is
	// roughly Binomial(c, s/bigS) with stdev ≈ √(c·(s/bigS)·(1−s/bigS)) ≈ 9.5,
	// so a single stream landing 3-4σ away (≈30-40 off 100) is expected.
	// Tolerance is generous enough to absorb that without flaking but still
	// flags degenerate distributions (always-the-same subset).
	const expected = c * s / bigS // 100
	const tolerance = 60
	for i := 1; i <= bigS; i++ {
		name := strconv.Itoa(i)
		hits := streamHits[name]
		assert.InDelta(t, expected, hits, tolerance,
			"stream %q appeared in %d clients; expected ~%d (±%d)", name, hits, expected, tolerance)
	}
}

func TestBuildPool_SetupFailureUnderTolerance(t *testing.T) {
	srv, err := lib.BuildServer(lib.ServerConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	// One client subscribes to an empty stream name, which the server
	// rejects with a `reject_subscription` envelope. The other nine each
	// pick a valid singleton stream.
	streams := make([][]string, 10)
	for i := range streams {
		streams[i] = []string{"s-" + strconv.Itoa(i)}
	}
	streams[5] = []string{""} // server rejects this identifier

	pool, err := lib.BuildPool(lib.PoolConfig{
		ServerURL: srv.WebSocketURL(),
		C:         10,
		S:         1,
		BigS:      1,
		Tolerance: 1,
		Streams:   streams,
	})
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	assert.Equal(t, 9, pool.Size(), "one failure tolerated; nine clients should remain")
}

func TestBuildPool_SetupFailureOverTolerance(t *testing.T) {
	srv, err := lib.BuildServer(lib.ServerConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	streams := make([][]string, 5)
	for i := range streams {
		streams[i] = []string{"s-" + strconv.Itoa(i)}
	}
	streams[2] = []string{""}

	pool, err := lib.BuildPool(lib.PoolConfig{
		ServerURL: srv.WebSocketURL(),
		C:         5,
		S:         1,
		BigS:      1,
		Tolerance: 0,
		Streams:   streams,
	})
	require.Error(t, err)
	assert.Nil(t, pool)
	assert.Contains(t, err.Error(), "client 2", "error must name the failed client")
	assert.Contains(t, err.Error(), "subscribe", "error must indicate the failing phase")
}
