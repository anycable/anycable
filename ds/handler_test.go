package ds

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mocks"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/streams"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDSHandler_HEAD(t *testing.T) {
	appNode, brk, streamCtrl := buildNode()
	conf := NewConfig()
	conf.Path = "/ds"
	conf.SkipAuth = true

	defer appNode.Shutdown(context.Background()) // nolint: errcheck

	headersExtractor := &server.DefaultHeadersExtractor{}

	handler := DSHandler(appNode, brk, streamCtrl, nil, context.Background(), headersExtractor, &conf, slog.Default())

	t.Run("returns stream metadata", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("HEAD", "/ds/test-stream", nil)

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		assert.NotEmpty(t, w.Header().Get(StreamOffsetHeader))
	})

	t.Run("requires stream path", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("HEAD", "/ds/", nil)

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("stream authorization", func(t *testing.T) {
		w := httptest.NewRecorder()

		streamCtrl := streams.NewController("", func(identifier string) (*streams.SubscribeRequest, error) {
			var resolved map[string]string

			err := json.Unmarshal([]byte(identifier), &resolved)
			require.NoError(t, err)

			assert.Equal(t, "test-stream", resolved["stream_name"])

			if resolved["signed_stream_name"] == "" {
				return nil, errors.New("signed stream name is missing")
			} else {
				assert.Equal(t, "s1t2r3e4a5m", resolved["signed_stream_name"])
			}

			return &streams.SubscribeRequest{
				StreamName: resolved["stream_name"],
			}, nil
		}, slog.Default())

		handler := DSHandler(appNode, brk, streamCtrl, nil, context.Background(), headersExtractor, &conf, slog.Default())

		req, _ := http.NewRequest("HEAD", "/ds/test-stream?signed=s1t2r3e4a5m", nil)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		w = httptest.NewRecorder()
		req, _ = http.NewRequest("HEAD", "/ds/test-stream", nil)
		req.Header.Set("X-Signed", "s1t2r3e4a5m")

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		w = httptest.NewRecorder()
		req, _ = http.NewRequest("HEAD", "/ds/test-stream", nil)

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestDSHandler_GET(t *testing.T) {
	appNode, brk, streamCtrl := buildNode()
	conf := NewConfig()
	conf.Path = "/ds"
	conf.SkipAuth = true

	defer appNode.Shutdown(context.Background()) // nolint: errcheck

	headersExtractor := &server.DefaultHeadersExtractor{}
	handler := DSHandler(appNode, brk, streamCtrl, nil, context.Background(), headersExtractor, &conf, slog.Default())

	t.Run("catch-up mode w/ empty stream", func(t *testing.T) {
		brk.
			On("HistorySince", "test-stream", int64(0)).
			Return([]common.StreamMessage{}, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ds/test-stream", nil)

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		assert.NotEmpty(t, w.Header().Get(StreamOffsetHeader))

		assert.Equal(t, "[]", w.Body.String())
	})

	t.Run("catch-up with valid offset", func(t *testing.T) {
		brk.
			On("HistoryFrom", "test-stream", "epoch1", uint64(10)).
			Return([]common.StreamMessage{
				{
					Data: `{"id":1}`,
				},
				{
					Data: `{"id":2}`,
				},
			}, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ds/test-stream?offset=10::epoch1", nil)

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, `[{"id":1},{"id":2}]`, w.Body.String())
	})

	t.Run("catch-up with stale offset", func(t *testing.T) {
		brk.
			On("HistoryFrom", "test-stream", "poch", uint64(11)).
			Return(nil, errors.New("invalid offset"))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ds/test-stream?offset=11::poch", nil)

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusGone, w.Code)
	})

	t.Run("requires stream path", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ds/", nil)

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("validates live mode", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ds/test-stream?live=invalid", nil)

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("requires offset for live mode", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ds/test-stream?live=long-poll", nil)

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func buildNode() (*node.Node, *mocks.Broker, *streams.Controller) {
	controller := &mocks.Controller{}
	controller.
		On("Shutdown").
		Return(nil)

	config := node.NewConfig()
	config.HubGopoolSize = 2

	n := node.NewNode(&config, node.WithController(controller), node.WithInstrumenter(metrics.NewMetrics(nil, 10, slog.Default())))
	go n.Start() // nolint:errcheck

	brk := &mocks.Broker{}
	n.SetBroker(brk)

	streamCtrl := streams.NewController("", func(identifier string) (*streams.SubscribeRequest, error) {
		return &streams.SubscribeRequest{
			StreamName: "a",
		}, nil
	}, slog.Default())

	return n, brk, streamCtrl
}
