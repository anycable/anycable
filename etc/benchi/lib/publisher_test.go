package lib_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anycable/anycable-go/etc/benchi/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSlowHandler returns an httptest.Server that sleeps `delay` per request
// and responds 201. Used to simulate HTTP backpressure on the publisher.
func newSlowHandler(t *testing.T, delay time.Duration) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPublisher_IssuedAndDroppedCounts(t *testing.T) {
	srv := newSlowHandler(t, 100*time.Millisecond)

	// Pin to a single worker so the queue-full path is observable; with the
	// default worker count, every Publish here would be picked up before the
	// queue could fill.
	p := lib.NewPublisher(srv.URL, 2, lib.WithWorkers(1), lib.WithBatchSize(1))
	t.Cleanup(p.Close)

	start := time.Now()
	for range 10 {
		p.Publish("foo", "x")
	}
	enqueueElapsed := time.Since(start)
	assert.Less(t, enqueueElapsed, 50*time.Millisecond, "enqueueing 10 should be fast, not gated on HTTP latency")

	issued := p.IssuedCount()
	dropped := p.DroppedCount()
	assert.EqualValues(t, 10, issued+dropped, "issued+dropped must equal total Publish calls")
	assert.Greater(t, dropped, int64(0), "expected at least one drop with maxInflight=2 and 100ms-slow handler")
}

func TestPublisher_CadenceNotGatedByHTTPLatency(t *testing.T) {
	srv := newSlowHandler(t, 50*time.Millisecond)

	// maxInflight large enough that the queue never fills during the window —
	// we want to prove cadence is decoupled from HTTP latency, not that the
	// queue is small.
	p := lib.NewPublisher(srv.URL, 1024)
	t.Cleanup(p.Close)

	ticker := time.NewTicker(10 * time.Millisecond) // 100 Hz
	defer ticker.Stop()
	deadline := time.After(200 * time.Millisecond)

	var stop bool
	for !stop {
		select {
		case <-ticker.C:
			p.Publish("foo", "x")
		case <-deadline:
			stop = true
		}
	}

	issued := p.IssuedCount()
	assert.GreaterOrEqual(t, issued, int64(15), "expected ~20 ticks in 200ms; got %d (cadence may be gated on HTTP)", issued)
	assert.LessOrEqual(t, issued, int64(25), "expected ~20 ticks in 200ms; got %d (clock drift?)", issued)
	assert.EqualValues(t, 0, p.DroppedCount(), "queue should not have dropped any with maxInflight=1024")
}

func TestPublisher_TargetCountsSnapshot(t *testing.T) {
	srv := newSlowHandler(t, 0)

	p := lib.NewPublisher(srv.URL, 16)
	for _, s := range []string{"a", "b", "a", "c"} {
		p.Publish(s, "x")
	}
	p.Close()

	counts := p.TargetCounts()
	assert.Equal(t, map[string]int{"a": 2, "b": 1, "c": 1}, counts)
}

func TestPublisher_ClosedThenPublishIsNoOp(t *testing.T) {
	srv := newSlowHandler(t, 0)

	p := lib.NewPublisher(srv.URL, 4)
	p.Publish("a", "x")
	// Let the worker drain so issued+dropped accounting after Close is
	// deterministic regardless of timing.
	require.Eventually(t, func() bool { return p.IssuedCount() == 1 }, time.Second, 5*time.Millisecond)
	p.Close()

	issuedBefore := p.IssuedCount()
	droppedBefore := p.DroppedCount()

	assert.NotPanics(t, func() {
		p.Publish("a", "x")
	})

	assert.Equal(t, issuedBefore, p.IssuedCount(), "Publish after Close must not increase issued")
	assert.Equal(t, droppedBefore+1, p.DroppedCount(), "Publish after Close is counted as dropped")
}

func TestPublisher_EmptyStream(t *testing.T) {
	srv := newSlowHandler(t, 0)

	p := lib.NewPublisher(srv.URL, 4)
	t.Cleanup(p.Close)

	assert.NotPanics(t, func() {
		p.Publish("", "x")
	})
}

func TestPublisher_HandlerReturns500_RecordsAsIssuedNotDropped(t *testing.T) {
	var broadcastsSeen atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var arr []map[string]any
		if json.Unmarshal(body, &arr) == nil {
			broadcastsSeen.Add(int64(len(arr)))
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	p := lib.NewPublisher(srv.URL, 16)
	for range 5 {
		p.Publish("a", "x")
	}
	p.Close()

	assert.EqualValues(t, 5, p.IssuedCount(), "issued counts all enqueued tasks, regardless of HTTP status")
	assert.EqualValues(t, 0, p.DroppedCount(), "non-2xx is not a drop — drops only count scheduler-fell-behind")
	assert.EqualValues(t, 0, p.CompletedCount(), "non-2xx must not increment completed")
	assert.EqualValues(t, 5, broadcastsSeen.Load(), "server should have received all 5 broadcasts (possibly batched into fewer POSTs)")
}

func TestPublisher_CompletedCountTracks2xx(t *testing.T) {
	srv := newSlowHandler(t, 0)

	p := lib.NewPublisher(srv.URL, 16)
	for range 7 {
		p.Publish("a", "x")
	}
	p.Close()

	assert.EqualValues(t, 7, p.IssuedCount())
	assert.EqualValues(t, 7, p.CompletedCount(), "all 2xx-responded broadcasts should be completed")
}

func TestPublisher_BatchingCoalescesUnderBackpressure(t *testing.T) {
	// One slow worker; production pushes 20 tasks faster than dispatch.
	// The single worker will pack subsequent tasks into batches up to the
	// configured batchSize, so we should see far fewer POSTs than tasks.
	var posts atomic.Int64
	var broadcasts atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		body, _ := io.ReadAll(r.Body)
		var arr []map[string]any
		if json.Unmarshal(body, &arr) == nil {
			broadcasts.Add(int64(len(arr)))
		}
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)

	p := lib.NewPublisher(srv.URL, 1024, lib.WithWorkers(1), lib.WithBatchSize(64))
	for range 20 {
		p.Publish("a", "x")
	}
	p.Close()

	assert.EqualValues(t, 20, p.IssuedCount())
	assert.EqualValues(t, 20, broadcasts.Load(), "server must still see every broadcast, just packed")
	assert.EqualValues(t, 20, p.CompletedCount())
	assert.Less(t, posts.Load(), int64(20), "batching should yield fewer POSTs than broadcasts under backpressure")
}

func TestPublisher_AgainstBuildServer(t *testing.T) {
	srv, err := lib.BuildServer(lib.ServerConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	p := lib.NewPublisher(srv.BroadcastURL(), 4)

	p.Publish("foo", "hello")

	// Wait for the in-flight task to be dispatched. We can't observe the
	// server-side delivery without a subscriber (lands in U4), so we settle
	// for "publisher round-tripped without dropping the task and the worker
	// drained" — the integration test in U6 covers the end-to-end deliver.
	require.Eventually(t, func() bool {
		return p.IssuedCount() == 1
	}, time.Second, 5*time.Millisecond)

	p.Close()

	assert.EqualValues(t, 1, p.IssuedCount())
	assert.EqualValues(t, 0, p.DroppedCount())

	// Sanity-check that Close-then-second-Close is fine — keeps the integration
	// test from leaking goroutines into later tests.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.Close()
	}()
	wg.Wait()
}
