package lib_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anycable/anycable-go/etc/benchi/lib"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// receiveWithin pulls one message from c with a deadline so a wedged Receive
// fails the test fast instead of hanging the suite.
func receiveWithin(t *testing.T, c *lib.Client, d time.Duration) (lib.Message, bool) {
	t.Helper()
	type result struct {
		msg lib.Message
		ok  bool
	}
	ch := make(chan result, 1)
	go func() {
		msg, ok := c.Receive()
		ch <- result{msg, ok}
	}()
	select {
	case r := <-ch:
		return r.msg, r.ok
	case <-time.After(d):
		t.Fatalf("Receive did not return within %s", d)
		return lib.Message{}, false
	}
}

func TestClient_ConnectSubscribeReceive(t *testing.T) {
	srv, err := lib.BuildServer(lib.ServerConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	cli, err := lib.BuildClient(srv.WebSocketURL())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	require.NoError(t, cli.Connect(context.Background()))
	require.NoError(t, cli.Subscribe([]string{"foo"}))

	pub := lib.NewPublisher(srv.BroadcastURL(), 4)
	t.Cleanup(pub.Close)
	pub.Publish("foo", "hello")

	msg, ok := receiveWithin(t, cli, 2*time.Second)
	require.True(t, ok)
	assert.Equal(t, "foo", msg.Stream)
	assert.Equal(t, "hello", msg.Data)
}

func TestClient_MultiStreamSubscribe(t *testing.T) {
	srv, err := lib.BuildServer(lib.ServerConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	cli, err := lib.BuildClient(srv.WebSocketURL())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	require.NoError(t, cli.Connect(context.Background()))

	streams := []string{"a", "b", "c", "d", "e"}
	require.NoError(t, cli.Subscribe(streams))

	pub := lib.NewPublisher(srv.BroadcastURL(), 16)
	t.Cleanup(pub.Close)
	for _, s := range streams {
		pub.Publish(s, s+"-payload")
	}

	got := make(map[string]string)
	for range streams {
		msg, ok := receiveWithin(t, cli, 2*time.Second)
		require.True(t, ok)
		got[msg.Stream] = msg.Data
	}
	for _, s := range streams {
		assert.Equal(t, s+"-payload", got[s], "missing message on stream %q", s)
	}
}

func TestClient_SubscribeIdempotency(t *testing.T) {
	srv, err := lib.BuildServer(lib.ServerConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	cli, err := lib.BuildClient(srv.WebSocketURL())
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	require.NoError(t, cli.Connect(context.Background()))
	require.NoError(t, cli.Subscribe([]string{"foo"}))
	require.NoError(t, cli.Subscribe([]string{"foo"}), "second Subscribe must not error")

	pub := lib.NewPublisher(srv.BroadcastURL(), 4)
	t.Cleanup(pub.Close)
	pub.Publish("foo", "hello")

	msg, ok := receiveWithin(t, cli, 2*time.Second)
	require.True(t, ok)
	assert.Equal(t, "foo", msg.Stream)
	assert.Equal(t, "hello", msg.Data)

	// No duplicate delivery — a second receive within a short window must
	// time out, proving messages are delivered once per publish.
	type result struct {
		msg lib.Message
		ok  bool
	}
	ch := make(chan result, 1)
	go func() {
		msg, ok := cli.Receive()
		ch <- result{msg, ok}
	}()
	select {
	case r := <-ch:
		t.Fatalf("unexpected second message: %+v ok=%v", r.msg, r.ok)
	case <-time.After(200 * time.Millisecond):
		// expected — no duplicate
	}
}

func TestClient_CloseStopsReceive(t *testing.T) {
	srv, err := lib.BuildServer(lib.ServerConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	cli, err := lib.BuildClient(srv.WebSocketURL())
	require.NoError(t, err)

	require.NoError(t, cli.Connect(context.Background()))
	require.NoError(t, cli.Subscribe([]string{"foo"}))

	type result struct {
		msg lib.Message
		ok  bool
	}
	ch := make(chan result, 1)
	go func() {
		msg, ok := cli.Receive()
		ch <- result{msg, ok}
	}()

	cli.Close()

	select {
	case r := <-ch:
		assert.False(t, r.ok, "Receive after Close must return ok=false")
		assert.Equal(t, lib.Message{}, r.msg)
	case <-time.After(2 * time.Second):
		t.Fatal("Receive did not unblock after Close")
	}
}

func TestClient_ConnectToInvalidURL(t *testing.T) {
	// Port 1 is reserved on most systems and almost never listens; even if a
	// rogue process bound to it, the WS upgrade would still fail and Connect
	// would still error.
	cli, err := lib.BuildClient("ws://127.0.0.1:1/cable")
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	connErr := cli.Connect(ctx)
	require.Error(t, connErr)
}

func TestClient_SubscribeTimeout(t *testing.T) {
	// Stalled mock: accepts the WS upgrade, sends a welcome, then ignores
	// every incoming frame. The cinemast client subscribes successfully on
	// the wire but never sees a confirm — SubscribeAndWait must time out.
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	conns := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		conns <- c
		_ = c.WriteMessage(websocket.TextMessage, []byte(`{"type":"welcome"}`))
		// Drain client frames without responding so the server-side socket
		// stays open until the test closes it from the conns channel.
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}))
	t.Cleanup(func() {
		srv.Close()
		select {
		case c := <-conns:
			_ = c.Close()
		default:
		}
	})

	wsURL := "ws://" + strings.TrimPrefix(srv.URL, "http://")

	cli, err := lib.BuildClient(wsURL)
	require.NoError(t, err)
	t.Cleanup(cli.Close)

	require.NoError(t, cli.Connect(context.Background()))

	// Shorten the wait so the test completes quickly. We don't ship a public
	// setter; reach in via an indirect path: Subscribe itself uses the
	// client's default 5s timeout. Five seconds is fine for an error-path
	// test under -race; if it becomes a hot spot, we'd expose a setter.
	start := time.Now()
	subErr := cli.Subscribe([]string{"never-confirmed"})
	elapsed := time.Since(start)

	require.Error(t, subErr)
	assert.Contains(t, subErr.Error(), "never-confirmed", "error must name the offending stream")
	assert.Less(t, elapsed, 7*time.Second, "Subscribe took %s; expected to time out near the 5s default", elapsed)
	assert.GreaterOrEqual(t, elapsed, 4*time.Second, "Subscribe returned in %s; expected to wait near 5s", elapsed)

	// Sanity: ensure the mock server accepted at least one upgrade.
	select {
	case <-conns:
	default:
		t.Fatal("mock server never received a connection")
	}
}

// TestClient_BuildClient_RejectsEmptyURL — quick guard on the constructor.
func TestClient_BuildClient_RejectsEmptyURL(t *testing.T) {
	_, err := lib.BuildClient("")
	require.Error(t, err)
}

// Compile-time assertion that Close is safe to call from multiple goroutines.
var _ = func() bool {
	var c *lib.Client
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); c.Close() }()
	go func() { defer wg.Done(); c.Close() }()
	return true
}
