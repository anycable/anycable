package ds

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mocks"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/anycable/anycable-go/server"
	durablestreams "github.com/durable-streams/durable-streams/packages/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDSIntegration_Head(t *testing.T) {
	ctx := context.Background()

	n, _, ts := setupIntegrationServer(t)
	defer ts.Close()
	defer n.Shutdown(ctx) // nolint: errcheck

	client := durablestreams.NewClient(durablestreams.WithBaseURL(ts.URL))
	stream := client.Stream("/ds/test-stream")

	meta, err := stream.Head(ctx)
	require.NoError(t, err)

	assert.Equal(t, "application/json", meta.ContentType)
	assert.NotEmpty(t, meta.NextOffset)
}

func TestDSIntegration_CatchupRead(t *testing.T) {
	ctx := context.Background()

	n, brk, ts := setupIntegrationServer(t)
	defer ts.Close()
	defer n.Shutdown(ctx) // nolint: errcheck

	t.Run("with empty stream", func(t *testing.T) {
		client := durablestreams.NewClient(durablestreams.WithBaseURL(ts.URL))
		stream := client.Stream("/ds/empty-stream")

		it := stream.Read(ctx, durablestreams.WithOffset(durablestreams.StartOffset))
		defer it.Close()

		chunk, err := it.Next()
		if err == durablestreams.Done {
			t.Fatal("expected data but got Done")
		}
		require.NoError(t, err)

		assert.Equal(t, "[]", string(chunk.Data))
		assert.True(t, chunk.UpToDate)
	})

	t.Run("with non-empty stream", func(t *testing.T) {
		streamName := "test-stream-data"

		brk.Subscribe(streamName)
		err := brk.HandleBroadcast(&common.StreamMessage{
			Stream: streamName,
			Data:   `{"id":1,"msg":"hello"}`,
		})
		require.NoError(t, err)

		err = brk.HandleBroadcast(&common.StreamMessage{
			Stream: streamName,
			Data:   `{"id":2,"msg":"world"}`,
		})
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		client := durablestreams.NewClient(durablestreams.WithBaseURL(ts.URL))
		stream := client.Stream("/ds/" + streamName)

		it := stream.Read(ctx, durablestreams.WithOffset(durablestreams.StartOffset))
		defer it.Close()

		chunk, err := it.Next()
		if err == durablestreams.Done {
			t.Fatal("expected data but got Done")
		}
		require.NoError(t, err)

		// Parse the JSON array
		var messages []map[string]interface{}
		err = json.Unmarshal(chunk.Data, &messages)
		require.NoError(t, err)

		assert.Len(t, messages, 2)
		assert.Equal(t, float64(1), messages[0]["id"])
		assert.Equal(t, "hello", messages[0]["msg"])
		assert.Equal(t, float64(2), messages[1]["id"])
		assert.Equal(t, "world", messages[1]["msg"])

		assert.True(t, chunk.UpToDate)
		assert.NotEmpty(t, chunk.NextOffset)
	})

	t.Run("with offset", func(t *testing.T) {
		streamName := "test-stream-offset"
		brk.Subscribe(streamName)

		brk.HandleBroadcast(&common.StreamMessage{ // nolint: errcheck
			Stream: streamName,
			Data:   `{"id":1,"msg":"first"}`,
		})
		brk.HandleBroadcast(&common.StreamMessage{ // nolint: errcheck
			Stream: streamName,
			Data:   `{"id":2,"msg":"second"}`,
		})
		brk.HandleBroadcast(&common.StreamMessage{ // nolint: errcheck
			Stream: streamName,
			Data:   `{"id":3,"msg":"third"}`,
		})

		time.Sleep(50 * time.Millisecond)

		client := durablestreams.NewClient(durablestreams.WithBaseURL(ts.URL))
		stream := client.Stream("/ds/" + streamName)

		it := stream.Read(ctx, durablestreams.WithOffset(durablestreams.StartOffset))
		chunk, err := it.Next()
		require.NoError(t, err)
		it.Close()

		firstOffset := chunk.NextOffset
		t.Logf("First offset after initial read: %s", firstOffset)

		brk.HandleBroadcast(&common.StreamMessage{ // nolint: errcheck
			Stream: streamName,
			Data:   `{"id":4,"msg":"fourth"}`,
		})

		time.Sleep(50 * time.Millisecond)

		it2 := stream.Read(ctx, durablestreams.WithOffset(firstOffset))
		defer it2.Close()

		chunk2, err := it2.Next()
		if err == durablestreams.Done {
			t.Fatal("No new messages - offset may be at end of stream")
		}
		require.NoError(t, err)

		var messages []map[string]interface{}
		err = json.Unmarshal(chunk2.Data, &messages)
		require.NoError(t, err)

		t.Logf("Messages received: %v", messages)

		require.NotEmpty(t, messages)
		lastMsg := messages[len(messages)-1]
		assert.Equal(t, float64(4), lastMsg["id"])
		assert.Equal(t, "fourth", lastMsg["msg"])
	})
}

func TestDSIntegration_AuthenticationRequired(t *testing.T) {
	t.Skip("no authentication yet")

	ctx := context.Background()

	n, _, ts := setupIntegrationServer(t)
	defer ts.Close()
	defer n.Shutdown(ctx) // nolint: errcheck

	client := durablestreams.NewClient(durablestreams.WithBaseURL(ts.URL))
	stream := client.Stream("/ds/test-stream")

	_, err := stream.Head(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestDSIntegration_LongPoll(t *testing.T) {
	ctx := context.Background()

	n, brk, ts := setupIntegrationServer(t)
	defer ts.Close()
	defer n.Shutdown(ctx) // nolint: errcheck

	t.Run("should wait for new data with long-poll", func(t *testing.T) {
		t.Skip("TODO: Long-poll mode is not yet implemented")

		streamName := "longpoll-wait-test"
		brk.Subscribe(streamName)

		client := durablestreams.NewClient(durablestreams.WithBaseURL(ts.URL))
		stream := client.Stream("/ds/" + streamName)

		// Get current offset first
		meta, err := stream.Head(ctx)
		require.NoError(t, err)

		var receivedData []map[string]interface{}
		var readErr error
		var wg sync.WaitGroup
		wg.Add(1)

		// Start reading in long-poll mode
		go func() {
			defer wg.Done()

			readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			it := stream.Read(readCtx,
				durablestreams.WithOffset(meta.NextOffset),
				durablestreams.WithLive(durablestreams.LiveModeLongPoll),
			)
			defer it.Close()

			chunk, itErr := it.Next()
			if itErr != nil {
				if itErr != durablestreams.Done {
					readErr = itErr
				}
				return
			}

			if len(chunk.Data) > 0 {
				var messages []map[string]interface{}
				if jerr := json.Unmarshal(chunk.Data, &messages); jerr == nil {
					receivedData = messages
				}
			}
		}()

		// Wait a bit for the long-poll to be active
		time.Sleep(500 * time.Millisecond)

		// Append data while long-poll is waiting
		err = brk.HandleBroadcast(&common.StreamMessage{
			Stream: streamName,
			Data:   `{"msg":"new data"}`,
		})
		require.NoError(t, err)

		wg.Wait()

		if readErr != nil {
			t.Logf("Long-poll read error: %v", readErr)
		}
		require.NoError(t, readErr)
		require.NotEmpty(t, receivedData, "should have received data via long-poll")
		assert.Equal(t, "new data", receivedData[0]["msg"])
	})

	t.Run("should return immediately if data already exists", func(t *testing.T) {
		streamName := "longpoll-immediate-test"
		brk.Subscribe(streamName)

		// Add data first
		err := brk.HandleBroadcast(&common.StreamMessage{
			Stream: streamName,
			Data:   `{"msg":"existing data"}`,
		})
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		client := durablestreams.NewClient(durablestreams.WithBaseURL(ts.URL))
		stream := client.Stream("/ds/" + streamName)

		// Read should return existing data immediately (no live mode)
		startTime := time.Now()

		it := stream.Read(ctx, durablestreams.WithOffset(durablestreams.StartOffset))
		defer it.Close()

		chunk, err := it.Next()
		require.NoError(t, err)

		elapsed := time.Since(startTime)

		// Should return quickly (not waiting for timeout)
		assert.Less(t, elapsed, 2*time.Second, "should return immediately without waiting")

		var messages []map[string]interface{}
		err = json.Unmarshal(chunk.Data, &messages)
		require.NoError(t, err)

		require.NotEmpty(t, messages)
		assert.Equal(t, "existing data", messages[0]["msg"])
	})
}

func setupIntegrationServer(t *testing.T) (*node.Node, broker.Broker, *httptest.Server) {
	t.Helper()

	config := node.NewConfig()
	config.HubGopoolSize = 2
	config.DisconnectMode = node.DISCONNECT_MODE_NEVER

	controller := &mocks.Controller{}
	controller.On("Shutdown").Return(nil)

	logger := slog.Default()
	n := node.NewNode(&config, node.WithController(controller), node.WithInstrumenter(metrics.NewMetrics(nil, 10, logger)))
	n.SetDisconnector(node.NewNoopDisconnector())

	bconf := broker.NewConfig()
	bconf.HistoryTTL = 60
	bconf.HistoryLimit = 100

	subscriber := pubsub.NewLegacySubscriber(n)
	brk := broker.NewMemoryBroker(subscriber, n, &bconf)
	brk.SetEpoch("test-epoch")
	n.SetBroker(brk)

	require.NoError(t, brk.Start(nil))

	go n.Start() // nolint:errcheck

	dsConfig := NewConfig()
	dsConfig.Path = "/ds"

	headersExtractor := &server.DefaultHeadersExtractor{}
	handler := DSHandler(n, brk, nil, context.Background(), headersExtractor, &dsConfig, logger)

	mux := http.NewServeMux()
	mux.Handle("/ds/", handler)

	ts := httptest.NewServer(mux)

	return n, brk, ts
}
