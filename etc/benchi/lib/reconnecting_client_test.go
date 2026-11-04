package lib

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/anycable/anycable-go/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildRPCServer(t *testing.T, rpcCfg RPCServerConfig, configure func(*config.Config)) (*Server, *RPCServer) {
	t.Helper()
	rpc := NewRPCServer(rpcCfg)
	t.Cleanup(rpc.Close)

	srv, err := BuildServer(ServerConfig{RPCHost: rpc.URL(), Configure: configure})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })
	return srv, rpc
}

func startClients(t *testing.T, n int, cfg ReconnectingClientConfig) []*ReconnectingClient {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	clients := make([]*ReconnectingClient, n)
	for i := range clients {
		cfg.Seed = uint64(i)
		clients[i] = NewReconnectingClient(cfg)
		wg.Add(1)
		go func(c *ReconnectingClient) {
			defer wg.Done()
			c.Run(ctx, 0)
		}(clients[i])
	}
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})
	return clients
}

var benchChannels = []string{
	`{"channel":"BenchChannel","id":"1"}`,
	`{"channel":"BenchChannel","id":"2"}`,
}

func TestReconnectingClient_BecomesHealthyAndRecoversFromDrop(t *testing.T) {
	srv, rpc := buildRPCServer(t, RPCServerConfig{Latency: 5 * time.Millisecond, Jitter: 0.5}, nil)

	stats := &ClientStats{}
	clients := startClients(t, 5, ReconnectingClientConfig{
		URL:                    srv.WebSocketURL(),
		Channels:               benchChannels,
		SubscribeRetryInterval: 5 * time.Second,
		PingTimeout:            6 * time.Second,
		Backoff:                Backoff{Min: 10 * time.Millisecond, Max: 100 * time.Millisecond},
		Stats:                  stats,
	})

	require.Eventually(t, func() bool { return stats.State(StateHealthy) == 5 }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, int64(5), rpc.Stats(RPCConnect).Completed)
	assert.Equal(t, int64(10), rpc.Stats(RPCCommand).Completed)
	assert.Equal(t, int64(5), stats.Welcome.Snapshot().Count)
	assert.Equal(t, int64(10), stats.Confirm.Snapshot().Count)

	require.True(t, clients[0].Drop())

	// The client records the close reason before it leaves the healthy
	// state, so wait for the reconnect attempt too
	require.Eventually(t, func() bool {
		return stats.Reason(ReasonDropped) == 1 && stats.Attempts() == 6 && stats.State(StateHealthy) == 5
	}, 5*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return rpc.Stats(RPCDisconnect).Completed == 1 }, 5*time.Second, 10*time.Millisecond)
}

func TestReconnectingClient_SubscribeTimeout(t *testing.T) {
	srv, _ := buildRPCServer(t, RPCServerConfig{CommandLatency: time.Second}, nil)

	stats := &ClientStats{}
	startClients(t, 1, ReconnectingClientConfig{
		URL:                    srv.WebSocketURL(),
		Channels:               benchChannels,
		SubscribeRetryInterval: 100 * time.Millisecond,
		PingTimeout:            6 * time.Second,
		Backoff:                Backoff{Min: time.Hour},
		Stats:                  stats,
	})

	require.Eventually(t, func() bool { return stats.Reason(ReasonSubscribeTimeout) == 1 }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, int64(1), stats.State(StateBackoff))
	assert.Equal(t, int64(0), stats.Confirm.Snapshot().Count)
	assert.Equal(t, int64(0), stats.SubscribeRetries())
}

func TestReconnectingClient_SubscribeRetriesThenReconnects(t *testing.T) {
	srv, _ := buildRPCServer(t, RPCServerConfig{CommandLatency: time.Second}, nil)

	stats := &ClientStats{}
	startClients(t, 1, ReconnectingClientConfig{
		URL:                    srv.WebSocketURL(),
		Channels:               benchChannels,
		SubscribeRetryInterval: 100 * time.Millisecond,
		SubscribeRetriesMax:    2,
		PingTimeout:            6 * time.Second,
		Backoff:                Backoff{Min: time.Hour},
		Stats:                  stats,
	})

	require.Eventually(t, func() bool { return stats.Reason(ReasonSubscribeTimeout) == 1 }, 5*time.Second, 10*time.Millisecond)
	// Both channels are re-sent twice before the client gives up
	assert.Equal(t, int64(4), stats.SubscribeRetries())
	assert.Equal(t, int64(1), stats.Attempts())
}

func TestReconnectingClient_SubscribeRetryConfirmed(t *testing.T) {
	srv, rpc := buildRPCServer(t, RPCServerConfig{CommandLatency: 300 * time.Millisecond}, nil)

	stats := &ClientStats{}
	startClients(t, 1, ReconnectingClientConfig{
		URL:                    srv.WebSocketURL(),
		Channels:               benchChannels[:1],
		SubscribeRetryInterval: 200 * time.Millisecond,
		SubscribeRetriesMax:    2,
		PingTimeout:            6 * time.Second,
		Backoff:                Backoff{Min: time.Hour},
		Stats:                  stats,
	})

	require.Eventually(t, func() bool { return stats.State(StateHealthy) == 1 }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, int64(1), stats.SubscribeRetries())

	// The re-sent command does not break the subscription
	time.Sleep(time.Second)
	assert.Equal(t, int64(1), stats.State(StateHealthy))
	assert.Equal(t, int64(1), stats.Attempts())
	assert.Equal(t, int64(1), stats.Confirm.Snapshot().Count)
	assert.Equal(t, int64(1), rpc.Stats(RPCCommand).Completed)
}

func TestReconnectingClient_SequentialSubscribe(t *testing.T) {
	for _, sequential := range []bool{false, true} {
		t.Run(fmt.Sprintf("sequential=%t", sequential), func(t *testing.T) {
			srv, _ := buildRPCServer(t, RPCServerConfig{CommandLatency: 200 * time.Millisecond}, nil)

			stats := &ClientStats{}
			startClients(t, 1, ReconnectingClientConfig{
				URL:                    srv.WebSocketURL(),
				Channels:               benchChannels,
				SequentialSubscribe:    sequential,
				SubscribeRetryInterval: 5 * time.Second,
				PingTimeout:            6 * time.Second,
				Backoff:                Backoff{Min: time.Hour},
				Stats:                  stats,
			})

			require.Eventually(t, func() bool { return stats.State(StateHealthy) == 1 }, 5*time.Second, 10*time.Millisecond)

			// The server handles a session's commands one at a time. When all
			// commands go out at once, the second one waits for the first.
			confirm := stats.Confirm.Snapshot()
			assert.Equal(t, int64(2), confirm.Count)
			if sequential {
				assert.Less(t, confirm.Max, 300*time.Millisecond)
			} else {
				assert.Greater(t, confirm.Max, 350*time.Millisecond)
			}
		})
	}
}

func TestReconnectingClient_PingTimeoutBeforeWelcome(t *testing.T) {
	srv, _ := buildRPCServer(t, RPCServerConfig{ConnectLatency: 2 * time.Second}, func(c *config.Config) {
		c.App.PingInterval = 10
	})

	stats := &ClientStats{}
	startClients(t, 1, ReconnectingClientConfig{
		URL:                    srv.WebSocketURL(),
		Channels:               benchChannels,
		SubscribeRetryInterval: 5 * time.Second,
		PingTimeout:            200 * time.Millisecond,
		Backoff:                Backoff{Min: time.Hour},
		Stats:                  stats,
	})

	require.Eventually(t, func() bool { return stats.Reason(ReasonPingTimeout) == 1 }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, int64(0), stats.Welcome.Snapshot().Count)
}

func TestReconnectingClient_Probe(t *testing.T) {
	srv, rpc := buildRPCServer(t, RPCServerConfig{Latency: 5 * time.Millisecond}, nil)

	stats := &ClientStats{}
	startClients(t, 1, ReconnectingClientConfig{
		URL:                    srv.WebSocketURL(),
		SubscribeRetryInterval: 5 * time.Second,
		PingTimeout:            6 * time.Second,
		ProbeInterval:          20 * time.Millisecond,
		Backoff:                Backoff{Min: time.Hour},
		Stats:                  stats,
	})

	require.Eventually(t, func() bool { return stats.Probe.Snapshot().Count >= 5 }, 5*time.Second, 10*time.Millisecond)
	assert.Less(t, stats.Probe.Snapshot().Max, time.Second)
	// The server answers pings without RPC calls.
	assert.Equal(t, int64(0), rpc.Stats(RPCCommand).Started)
}

func TestReconnectingClient_SessionIDs(t *testing.T) {
	stats := &ClientStats{}
	var mu sync.Mutex
	seen := map[string]int{}
	srv, _ := buildRPCServer(t, RPCServerConfig{
		Latency: 5 * time.Millisecond,
		SessionClosed: func(sid string) bool {
			mu.Lock()
			seen[sid]++
			mu.Unlock()
			return stats.SessionClosed(sid)
		},
	}, nil)

	clients := startClients(t, 1, ReconnectingClientConfig{
		URL:                    srv.WebSocketURL(),
		Channels:               benchChannels,
		SubscribeRetryInterval: 5 * time.Second,
		PingTimeout:            6 * time.Second,
		Backoff:                Backoff{Min: 10 * time.Millisecond},
		Stats:                  stats,
	})
	require.Eventually(t, func() bool { return stats.State(StateHealthy) == 1 }, 5*time.Second, 10*time.Millisecond)

	// One Connect and two Command calls carry the first session's ID
	mu.Lock()
	assert.Equal(t, map[string]int{"1": 3}, seen)
	mu.Unlock()
	assert.False(t, stats.SessionClosed("1"))

	require.True(t, clients[0].Drop())
	assert.True(t, stats.SessionClosed("1"))

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return seen["2"] == 3
	}, 5*time.Second, 10*time.Millisecond)
	assert.False(t, stats.SessionClosed("2"))
}

func TestHistogramQuantile(t *testing.T) {
	var h Histogram
	for i := 1; i <= 100; i++ {
		h.Record(time.Duration(i) * time.Millisecond)
	}
	s := h.Snapshot()
	assert.Equal(t, int64(100), s.Count)
	assert.InDelta(t, float64(50*time.Millisecond), float64(s.Quantile(0.5)), float64(3*time.Millisecond))
	assert.InDelta(t, float64(99*time.Millisecond), float64(s.Quantile(0.99)), float64(5*time.Millisecond))
	assert.Equal(t, 100*time.Millisecond, s.Quantile(1))

	h.Swap()
	assert.Equal(t, int64(0), h.Snapshot().Count)
}
