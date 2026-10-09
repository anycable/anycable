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

	handler, err := DSHandler(appNode, brk, streamCtrl, nil, context.Background(), headersExtractor, &conf, slog.Default())
	require.NoError(t, err)

	t.Run("returns stream metadata for unknown stream", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("HEAD", "/ds/test-stream", nil)

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		assert.Equal(t, "0", w.Header().Get(StreamOffsetHeader))
	})

	t.Run("returns stream metadata for existing stream", func(t *testing.T) {
		brk.On("Peak", "test-existing-stream").Return(
			&common.StreamMessage{
				Epoch:  "a321",
				Offset: 44,
			},
			nil,
		)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("HEAD", "/ds/test-existing-stream", nil)

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		assert.Equal(t, "44::a321", w.Header().Get(StreamOffsetHeader))
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

		handler, err := DSHandler(appNode, brk, streamCtrl, nil, context.Background(), headersExtractor, &conf, slog.Default())
		require.NoError(t, err)

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
	handler, err := DSHandler(appNode, brk, streamCtrl, nil, context.Background(), headersExtractor, &conf, slog.Default())
	require.NoError(t, err)

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
		assert.Equal(t, "public, max-age=60, stale-while-revalidate=300", w.Header().Get("Cache-Control"))
	})

	t.Run("catch-up with signed stream name in header", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ds/test-stream?offset=10::epoch1", nil)
		req.Header.Set(SignedStreamHeader, "s1t2r3e4a5m")

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "private, max-age=60, stale-while-revalidate=300", w.Header().Get("Cache-Control"))
	})

	t.Run("catch-up with now offset", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ds/test-stream?offset=now", nil)

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "[]", w.Body.String())
		assert.Equal(t, "true", w.Header().Get(StreamUpToDateHeader))
		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	})

	t.Run("catch-up with stale offset", func(t *testing.T) {
		brk.
			On("HistoryFrom", "test-stream", "poch", uint64(11)).
			Return(nil, errors.New("invalid offset"))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ds/test-stream?offset=11::poch", nil)

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusGone, w.Code)
		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
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

func TestReadCacheControl(t *testing.T) {
	const cacheable = "max-age=60, stale-while-revalidate=300"

	for _, tc := range []struct {
		name     string
		skipAuth bool
		url      string
		header   string
		expected string
	}{
		{"public stream w/o auth", true, "/ds/test?offset=-1", "", "public, " + cacheable},
		{"signed stream in query w/o auth", true, "/ds/test?offset=-1&signed=abc", "", "public, " + cacheable},
		{"signed stream in query and header w/o auth", true, "/ds/test?offset=-1&signed=abc", "abc", "public, " + cacheable},
		{"signed stream in header w/o auth", true, "/ds/test?offset=-1", "abc", "private, " + cacheable},
		{"with auth", false, "/ds/test?offset=-1&signed=abc", "", "private, " + cacheable},
		{"now offset", true, "/ds/test?offset=now", "", "no-store"},
		{"now offset in long-poll mode", true, "/ds/test?offset=now&live=long-poll", "", "no-store"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conf := NewConfig()
			conf.SkipAuth = tc.skipAuth

			req, _ := http.NewRequest("GET", tc.url, nil)
			if tc.header != "" {
				req.Header.Set(SignedStreamHeader, tc.header)
			}

			sp, err := StreamParamsFromReq(req, &conf)
			require.NoError(t, err)

			assert.Equal(t, tc.expected, readCacheControl(&conf, req, sp))
		})
	}
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
	brk.On("Peak", "test-stream").Return(nil, nil)
	n.SetBroker(brk)

	streamCtrl := streams.NewController("", func(identifier string) (*streams.SubscribeRequest, error) {
		var request streams.SubscribeRequest

		if err := json.Unmarshal([]byte(identifier), &request); err != nil {
			return nil, err
		}

		return &streams.SubscribeRequest{
			StreamName: request.StreamName,
		}, nil
	}, slog.Default())

	return n, brk, streamCtrl
}
