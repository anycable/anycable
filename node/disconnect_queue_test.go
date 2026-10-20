package node

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDisconnectQueue_Run(t *testing.T) {
	t.Run("Disconnects sessions", func(t *testing.T) {
		ctx := context.Background()
		q := newQueue()
		defer q.Shutdown(ctx) //nolint:errcheck

		assert.Nil(t, q.Enqueue(NewMockSession("1", q.node)))
		assert.Equal(t, 1, q.Size())

		go func() {
			if err := q.Run(); err != nil {
				panic(err)
			}
		}()

		// TODO: We need a trully node mock to control disconnect operation
		for q.Size() > 0 {
			runtime.Gosched()
		}
	})

	t.Run("Runs calls concurrently up to the number of workers", func(t *testing.T) {
		controller := &mocks.Controller{}
		config := NewConfig()
		node := NewNode(&config, WithInstrumenter(metrics.NewMetrics(nil, 10, slog.Default())), WithController(controller))

		release := make(chan struct{})
		var inflight, peak atomic.Int32
		var calls atomic.Int32

		controller.On("Disconnect", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(mock.Arguments) {
				n := inflight.Add(1)
				if n > peak.Load() {
					peak.Store(n)
				}
				<-release
				inflight.Add(-1)
				calls.Add(1)
			}).
			Return(nil)

		dconfig := NewDisconnectQueueConfig()
		dconfig.Rate = 1000
		dconfig.WorkersNum = 2
		q := NewDisconnectQueue(node, &dconfig, slog.Default())

		for i := range 3 {
			require.NoError(t, q.Enqueue(NewMockSession(strconv.Itoa(i), node)))
		}

		go q.Run() //nolint:errcheck

		require.Eventually(t, func() bool { return inflight.Load() == 2 }, time.Second, time.Millisecond)
		// The third call waits for a free worker
		time.Sleep(50 * time.Millisecond)
		assert.EqualValues(t, 2, inflight.Load())

		close(release)

		require.Eventually(t, func() bool { return calls.Load() == 3 }, time.Second, time.Millisecond)
		assert.EqualValues(t, 2, peak.Load())

		require.NoError(t, q.Shutdown(context.Background()))
	})

	t.Run("Respects the rate with multiple workers", func(t *testing.T) {
		controller := &mocks.Controller{}
		config := NewConfig()
		node := NewNode(&config, WithInstrumenter(metrics.NewMetrics(nil, 10, slog.Default())), WithController(controller))

		var calls atomic.Int32

		controller.On("Disconnect", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(mock.Arguments) { calls.Add(1) }).
			Return(nil)

		dconfig := NewDisconnectQueueConfig()
		dconfig.Rate = 20
		dconfig.WorkersNum = 4
		q := NewDisconnectQueue(node, &dconfig, slog.Default())

		for i := range 5 {
			require.NoError(t, q.Enqueue(NewMockSession(strconv.Itoa(i), node)))
		}

		start := time.Now()
		go q.Run() //nolint:errcheck

		require.Eventually(t, func() bool { return calls.Load() == 5 }, 2*time.Second, time.Millisecond)

		// 5 calls at 20 calls/s take at least 5 ticks of 50ms
		assert.GreaterOrEqual(t, time.Since(start), 240*time.Millisecond)

		require.NoError(t, q.Shutdown(context.Background()))
	})
}

func TestDisconnectQueue_Shutdown(t *testing.T) {
	t.Run("Disconnects sessions", func(t *testing.T) {
		ctx := context.Background()
		q := newQueue()

		assert.Nil(t, q.Enqueue(NewMockSession("1", q.node)))
		assert.Nil(t, q.Enqueue(NewMockSession("2", q.node)))
		assert.Equal(t, 2, q.Size())

		assert.Nil(t, q.Shutdown(ctx))
		assert.Equal(t, 0, q.Size())
	})

	t.Run("Stops after timeout", func(t *testing.T) {
		t.Skip("TODO: We need a trully node mock to control disconnect operation")
	})

	t.Run("Allows multiple entering", func(t *testing.T) {
		q := newQueue()
		ctx := context.Background()

		for i := 1; i <= 10; i++ {
			q.Shutdown(ctx) // nolint:errcheck
		}
	})

	t.Run("Does not block when sessions are enqueued concurrently", func(t *testing.T) {
		ctx := context.Background()

		for i := 0; i < 100; i++ {
			q := newQueue()

			for j := 0; j < 100; j++ {
				require.NoError(t, q.Enqueue(NewMockSession(strconv.Itoa(j), q.node)))
			}

			// Keep enqueueing until shutdown completes, so some Enqueue calls
			// pass the stopped check before Shutdown and add items during it
			stop := make(chan struct{})
			var wg sync.WaitGroup

			for j := 0; j < 8; j++ {
				wg.Add(1)
				go func() {
					defer wg.Done()

					for k := 0; ; k++ {
						select {
						case <-stop:
							return
						default:
						}

						q.Enqueue(NewMockSession(fmt.Sprintf("c-%d-%d", j, k), q.node)) // nolint:errcheck
					}
				}()
			}

			done := make(chan error, 1)

			go func() {
				done <- q.Shutdown(ctx)
			}()

			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("Shutdown is blocked")
			}

			close(stop)
			wg.Wait()
		}
	})
}

func TestDisconnectQueue_Enqueue(t *testing.T) {
	t.Run("Adds sessions to the queue", func(t *testing.T) {
		q := newQueue()

		assert.Nil(t, q.Enqueue(NewMockSession("1", q.node)))
		assert.Equal(t, 1, q.Size())
	})

	t.Run("After shutdown", func(t *testing.T) {
		ctx := context.Background()
		q := newQueue()
		q.Shutdown(ctx) // nolint:errcheck

		assert.Nil(t, q.Enqueue(NewMockSession("1", q.node)))
		assert.Equal(t, 0, q.Size())
	})

	t.Run("Discards sessions over the limit", func(t *testing.T) {
		node := NewMockNode()
		config := NewDisconnectQueueConfig()
		config.MaxBacklog = 2
		q := NewDisconnectQueue(node, &config, slog.Default())

		assert.Nil(t, q.Enqueue(NewMockSession("1", node)))
		assert.Nil(t, q.Enqueue(NewMockSession("2", node)))
		assert.Error(t, q.Enqueue(NewMockSession("3", node)))
		assert.Error(t, q.Enqueue(NewMockSession("4", node)))

		assert.Equal(t, 2, q.Size())
		assert.Equal(t, uint64(2), node.metrics.(*metrics.Metrics).Counter(metricsDisconnectDiscarded).Value())

		item, ok := q.disconnect.Remove()
		require.True(t, ok)
		assert.Equal(t, "1", item.Data.GetID())

		assert.Nil(t, q.Enqueue(NewMockSession("5", node)))
		assert.Equal(t, 2, q.Size())
	})

	t.Run("Does not limit the queue when max backlog is zero", func(t *testing.T) {
		node := NewMockNode()
		config := NewDisconnectQueueConfig()
		config.MaxBacklog = 0
		q := NewDisconnectQueue(node, &config, slog.Default())

		for i := 0; i < 1000; i++ {
			require.NoError(t, q.Enqueue(NewMockSession(strconv.Itoa(i), node)))
		}

		assert.Equal(t, 1000, q.Size())
		assert.Equal(t, uint64(0), node.metrics.(*metrics.Metrics).Counter(metricsDisconnectDiscarded).Value())
	})
}

func newQueue() *DisconnectQueue {
	node := NewMockNode()
	config := NewDisconnectQueueConfig()
	config.Rate = 1
	q := NewDisconnectQueue(node, &config, slog.Default())

	return q
}

func TestDisconnectQueueConfig_ToToml(t *testing.T) {
	conf := NewDisconnectQueueConfig()
	conf.Rate = 50
	conf.WorkersNum = 8
	conf.MaxBacklog = 2048

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "rate = 50")
	assert.Contains(t, tomlStr, "workers_num = 8")
	assert.Contains(t, tomlStr, "max_backlog = 2048")

	// Round-trip test
	conf2 := NewDisconnectQueueConfig()

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}
