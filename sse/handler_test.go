package sse

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mocks"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/anycable/anycable-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type streamingWriter struct {
	httptest.ResponseRecorder

	stream chan []byte
}

func newStreamingWriter(w *httptest.ResponseRecorder) *streamingWriter {
	return &streamingWriter{
		ResponseRecorder: *w,
		stream:           make(chan []byte, 100),
	}
}

func (w *streamingWriter) Write(data []byte) (int, error) {
	events := bytes.Split(data, []byte("\n\n"))

	for _, event := range events {
		if len(event) > 0 {
			w.stream <- event
		}
	}

	return w.ResponseRecorder.Write(data)
}

func (w *streamingWriter) ReadEvent(ctx context.Context) (string, error) {
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case event := <-w.stream:
			return string(event), nil
		}
	}
}

var _ http.ResponseWriter = (*streamingWriter)(nil)

func TestSSEHandler(t *testing.T) {
	appNode, controller := buildNode()
	conf := NewConfig()

	dconfig := node.NewDisconnectQueueConfig()
	dconfig.Rate = 1
	disconnector := node.NewDisconnectQueue(appNode, &dconfig)
	appNode.SetDisconnector(disconnector)

	go appNode.Start()                           // nolint: errcheck
	defer appNode.Shutdown(context.Background()) // nolint: errcheck

	headersExtractor := &server.DefaultHeadersExtractor{}

	handler := SSEHandler(appNode, context.Background(), headersExtractor, &conf)

	controller.
		On("Shutdown").
		Return(nil)

	controller.
		On("Disconnect", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	t.Run("headers", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)

		handler.ServeHTTP(w, req)

		assert.Equal(t, "text/event-stream; charset=utf-8", w.Header().Get("Content-Type"))
		assert.Equal(t, "private, no-cache, no-store, must-revalidate, max-age=0", w.Header().Get("Cache-Control"))
		assert.Equal(t, "no-cache", w.Header().Get("Pragma"))
		assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
	})

	t.Run("headers + CORS", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "http://www.example.com")

		corsConf := NewConfig()
		corsConf.AllowedOrigins = "*.example.com"

		corsHandler := SSEHandler(appNode, context.Background(), headersExtractor, &corsConf)

		corsHandler.ServeHTTP(w, req)

		assert.Equal(t, "http://www.example.com", w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("OPTIONS", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("OPTIONS", "/", nil)

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("non-GET/OPTIONS/POST", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/", nil)

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})

	t.Run("when authentication fails", func(t *testing.T) {
		defer assertNoSessions(t, appNode)

		controller.
			On("Authenticate", "sid-fail", mock.Anything).
			Return(&common.ConnectResult{
				Status:        common.FAILURE,
				Transmissions: []string{`{"type":"disconnect"}`},
			}, nil)

		req, _ := http.NewRequest("GET", "/?channel=room_1", nil)
		req.Header.Set("X-Request-ID", "sid-fail")

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Empty(t, w.Body.String())
	})

	t.Run("GET request with identifier", func(t *testing.T) {
		defer assertNoSessions(t, appNode)

		controller.
			On("Authenticate", "sid-gut", mock.Anything).
			Return(&common.ConnectResult{
				Identifier:    "se2023",
				Status:        common.SUCCESS,
				Transmissions: []string{`{"type":"welcome"}`},
			}, nil)

		controller.
			On("Subscribe", "sid-gut", mock.Anything, "se2023", "chat_1").
			Return(&common.CommandResult{
				Status:        common.SUCCESS,
				Transmissions: []string{`{"type":"confirm","identifier":"chat_1"}`},
				Streams:       []string{"messages_1"},
			}, nil)

		req, _ := http.NewRequest("GET", "/?identifier=chat_1", nil)
		req.Header.Set("X-Request-ID", "sid-gut")

		ctx_, release := context.WithTimeout(context.Background(), 2*time.Second)
		defer release()

		ctx, cancel := context.WithCancel(ctx_)
		defer cancel()

		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		sw := newStreamingWriter(w)

		go handler.ServeHTTP(sw, req)

		msg, err := sw.ReadEvent(ctx)
		require.NoError(t, err)
		assert.Equal(t, "event: welcome\n"+`data: {"type":"welcome"}`, msg)

		msg, err = sw.ReadEvent(ctx)
		require.NoError(t, err)
		assert.Equal(t, "event: confirm\n"+`data: {"type":"confirm","identifier":"chat_1"}`, msg)

		appNode.Broadcast(&common.StreamMessage{Stream: "messages_1", Data: `{"content":"hello"}`})

		msg, err = sw.ReadEvent(ctx)
		require.NoError(t, err)
		assert.Equal(t, `data: {"content":"hello"}`, msg)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GET request with turbo_signed_stream_name", func(t *testing.T) {
		defer assertNoSessions(t, appNode)

		controller.
			On("Authenticate", "sid-turbo", mock.Anything).
			Return(&common.ConnectResult{
				Identifier:    "se2023",
				Status:        common.SUCCESS,
				Transmissions: []string{`{"type":"welcome"}`},
			}, nil)

		turbo_identifier := `{"channel":"Turbo::StreamsChannel","signed_stream_name":"chat_1"}`

		controller.
			On("Subscribe", "sid-turbo", mock.Anything, "se2023", turbo_identifier).
			Return(&common.CommandResult{
				Status:        common.SUCCESS,
				Transmissions: []string{`{"type":"confirm","identifier":"turbo_1"}`},
				Streams:       []string{"chat_1"},
			}, nil)

		req, _ := http.NewRequest("GET", "/?turbo_signed_stream_name=chat_1", nil)
		req.Header.Set("X-Request-ID", "sid-turbo")

		ctx_, release := context.WithTimeout(context.Background(), 2*time.Second)
		defer release()

		ctx, cancel := context.WithCancel(ctx_)
		defer cancel()

		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		sw := newStreamingWriter(w)

		go handler.ServeHTTP(sw, req)

		msg, err := sw.ReadEvent(ctx)
		require.NoError(t, err)
		assert.Equal(t, "event: welcome\n"+`data: {"type":"welcome"}`, msg)

		msg, err = sw.ReadEvent(ctx)
		require.NoError(t, err)
		assert.Equal(t, "event: confirm\n"+`data: {"type":"confirm","identifier":"turbo_1"}`, msg)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GET request with channel + rejected", func(t *testing.T) {
		defer assertNoSessions(t, appNode)

		controller.
			On("Authenticate", "sid-reject", mock.Anything).
			Return(&common.ConnectResult{
				Identifier:    "se2034",
				Status:        common.SUCCESS,
				Transmissions: []string{`{"type":"welcome"}`},
			}, nil)

		controller.
			On("Subscribe", "sid-reject", mock.Anything, "se2034", `{"channel":"room_1"}`).
			Return(&common.CommandResult{
				Status:        common.FAILURE,
				Transmissions: []string{`{"type":"reject","identifier":"room_1"}`},
			}, nil)

		req, _ := http.NewRequest("GET", "/?channel=room_1", nil)
		req.Header.Set("X-Request-ID", "sid-reject")

		ctx_, release := context.WithTimeout(context.Background(), 2*time.Second)
		defer release()

		ctx, cancel := context.WithCancel(ctx_)
		defer cancel()

		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, w.Body.String())

		controller.AssertCalled(t, "Subscribe", "sid-reject", mock.Anything, "se2034", `{"channel":"room_1"}`)
	})

	t.Run("GET request without channel or identifier", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, w.Body.String())
	})

	t.Run("POST request without commands + server shutdown", func(t *testing.T) {
		defer assertNoSessions(t, appNode)

		controller.
			On("Authenticate", "sid-post-no-op", mock.Anything).
			Return(&common.ConnectResult{
				Identifier:    "se2023-09-06",
				Status:        common.SUCCESS,
				Transmissions: []string{`{"type":"welcome"}`},
			}, nil)

		req, _ := http.NewRequest("POST", "/", nil)
		req.Header.Set("X-Request-ID", "sid-post-no-op")

		ctx_, release := context.WithTimeout(context.Background(), 2*time.Second)
		defer release()

		ctx, cancel := context.WithCancel(ctx_)
		defer cancel()

		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		sw := newStreamingWriter(w)

		shutdownCtx, shutdownFn := context.WithCancel(context.Background())

		shutdownHandler := SSEHandler(appNode, shutdownCtx, headersExtractor, &conf)

		go shutdownHandler.ServeHTTP(sw, req)

		msg, err := sw.ReadEvent(ctx)
		require.NoError(t, err)
		assert.Equal(t, "event: welcome\n"+`data: {"type":"welcome"}`, msg)

		shutdownFn()

		msg, err = sw.ReadEvent(ctx)
		require.NoError(t, err)
		assert.Equal(t, "event: disconnect\n"+`data: {"type":"disconnect","reason":"server_restart","reconnect":true}`, msg)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("POST request with multiple subscriptions", func(t *testing.T) {
		defer assertNoSessions(t, appNode)

		controller.
			On("Authenticate", "sid-post", mock.Anything).
			Return(&common.ConnectResult{
				Identifier:    "se2023-09-06",
				Status:        common.SUCCESS,
				Transmissions: []string{`{"type":"welcome"}`},
			}, nil)

		controller.
			On("Subscribe", "sid-post", mock.Anything, "se2023-09-06", "chat_1").
			Return(&common.CommandResult{
				Status:        common.SUCCESS,
				Transmissions: []string{`{"type":"confirm","identifier":"chat_1"}`},
				Streams:       []string{"messages_1"},
			}, nil)

		controller.
			On("Subscribe", "sid-post", mock.Anything, "se2023-09-06", "presence_1").
			Return(&common.CommandResult{
				Status:        common.SUCCESS,
				Transmissions: []string{`{"type":"confirm","identifier":"presence_1"}`},
				Streams:       []string{"presence_1"},
			}, nil)

		req, _ := http.NewRequest("POST", "/", nil)
		req.Header.Set("X-Request-ID", "sid-post")
		req.Body = io.NopCloser(
			strings.NewReader("{\"command\":\"subscribe\",\"identifier\":\"chat_1\"}\n{\"command\":\"subscribe\",\"identifier\":\"presence_1\"}"),
		)

		ctx_, release := context.WithTimeout(context.Background(), 2*time.Second)
		defer release()

		ctx, cancel := context.WithCancel(ctx_)
		defer cancel()

		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		sw := newStreamingWriter(w)

		go handler.ServeHTTP(sw, req)

		msg, err := sw.ReadEvent(ctx)
		require.NoError(t, err)
		assert.Equal(t, "event: welcome\n"+`data: {"type":"welcome"}`, msg)

		msg, err = sw.ReadEvent(ctx)
		require.NoError(t, err)
		assert.Equal(t, "event: confirm\n"+`data: {"type":"confirm","identifier":"chat_1"}`, msg)

		msg, err = sw.ReadEvent(ctx)
		require.NoError(t, err)
		assert.Equal(t, "event: confirm\n"+`data: {"type":"confirm","identifier":"presence_1"}`, msg)

		appNode.Broadcast(&common.StreamMessage{Stream: "messages_1", Data: `{"content":"hello"}`})

		msg, err = sw.ReadEvent(ctx)
		require.NoError(t, err)
		assert.Equal(t, `data: {"identifier":"chat_1","message":{"content":"hello"}}`, msg)

		appNode.Broadcast(&common.StreamMessage{Stream: "presence_1", Data: `{"type":"join","user_id":1}`})

		msg, err = sw.ReadEvent(ctx)
		require.NoError(t, err)
		assert.Equal(t, `data: {"identifier":"presence_1","message":{"type":"join","user_id":1}}`, msg)

		require.Equal(t, http.StatusOK, w.Code)
	})
}

// This a helper method to ensure no sessions left after test (so no global state is left).
// Session may be removed from the hub asynchrounously, so we need to wait for it.
func assertNoSessions(t *testing.T, n *node.Node) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})

	go func() {
		for {
			if n.Size() == 0 {
				close(done)
				return
			}

			time.Sleep(100 * time.Millisecond)
		}
	}()

	select {
	case <-ctx.Done():
		require.Fail(t, "Timeout waiting for sessions to be removed")
	case <-done:
	}
}

type immediateDisconnector struct {
	n *node.Node
}

func (d *immediateDisconnector) Enqueue(s *node.Session) error {
	return d.n.DisconnectNow(s)
}

func (immediateDisconnector) Run() error                         { return nil }
func (immediateDisconnector) Shutdown(ctx context.Context) error { return nil }
func (immediateDisconnector) Size() int                          { return 0 }

func buildNode() (*node.Node, *mocks.Controller) {
	controller := &mocks.Controller{}
	config := node.NewConfig()
	config.HubGopoolSize = 2
	n := node.NewNode(controller, metrics.NewMetrics(nil, 10), &config)
	n.SetBroker(broker.NewLegacyBroker(pubsub.NewLegacySubscriber(n)))
	n.SetDisconnector(&immediateDisconnector{n})
	return n, controller
}
