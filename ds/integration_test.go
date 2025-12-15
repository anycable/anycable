package ds

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDSIntegration_Head(t *testing.T) {
	ctx := context.Background()

	n, _, controller, ts := setupIntegrationServer(t)
	defer ts.Close()
	defer n.Shutdown(ctx) // nolint: errcheck

	controller.
		On("Authenticate", mock.Anything, mock.Anything, mock.Anything).
		Return(&common.ConnectResult{
			Status:        common.SUCCESS,
			Transmissions: []string{`{"type":"welcome"}`},
		}, nil)

	client := durablestreams.NewClient(durablestreams.WithBaseURL(ts.URL))
	stream := client.Stream("/ds/test-stream")

	meta, err := stream.Head(ctx)
	require.NoError(t, err)

	assert.Equal(t, "application/json", meta.ContentType)
	assert.NotEmpty(t, meta.NextOffset)
}

func TestDSIntegration_CatchupRead(t *testing.T) {
	ctx := context.Background()

	n, brk, controller, ts := setupIntegrationServer(t)
	defer ts.Close()
	defer n.Shutdown(ctx) // nolint: errcheck

	controller.
		On("Authenticate", mock.Anything, mock.Anything, mock.Anything).
		Return(&common.ConnectResult{
			Status:        common.SUCCESS,
			Transmissions: []string{`{"type":"welcome"}`},
		}, nil)

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
	ctx := context.Background()

	n, _, controller, ts := setupIntegrationServer(t)
	defer ts.Close()
	defer n.Shutdown(ctx) // nolint: errcheck

	controller.
		On("Authenticate", mock.Anything, mock.Anything, mock.Anything).
		Return(&common.ConnectResult{
			Status:        common.FAILURE,
			Transmissions: []string{`{"type":"disconnect"}`},
		}, nil)

	client := durablestreams.NewClient(durablestreams.WithBaseURL(ts.URL))
	stream := client.Stream("/ds/test-stream")

	_, err := stream.Head(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func setupIntegrationServer(t *testing.T) (*node.Node, broker.Broker, *mocks.Controller, *httptest.Server) {
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

	return n, brk, controller, ts
}
