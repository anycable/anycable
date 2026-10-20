package node

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/netpoll"
	"github.com/anycable/anycable-go/utils"
	"github.com/anycable/anycable-go/ws"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingPoller records poller calls and keeps the callback to trigger events manually
type recordingPoller struct {
	mu       sync.Mutex
	calls    []string
	callback netpoll.CallbackFn
}

func (p *recordingPoller) Start(desc *netpoll.Desc, cb netpoll.CallbackFn) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.calls = append(p.calls, "start")
	p.callback = cb

	return nil
}

func (p *recordingPoller) Stop(desc *netpoll.Desc) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.calls = append(p.calls, "stop")

	return nil
}

func (p *recordingPoller) Resume(desc *netpoll.Desc) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.calls = append(p.calls, "resume")

	return nil
}

func (p *recordingPoller) WatchHangup(desc *netpoll.Desc) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.calls = append(p.calls, "watch")

	return nil
}

func (p *recordingPoller) Calls() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]string{}, p.calls...)
}

// blockingExecutor blocks HandleCommand until released or the session is closed (as RPC calls do)
type blockingExecutor struct {
	started  chan struct{}
	release  chan struct{}
	finished chan struct{}
}

func (e *blockingExecutor) HandleCommand(s *Session, msg *common.Message) error {
	close(e.started)
	select {
	case <-e.release:
	case <-s.closeCtx.Done():
	}
	close(e.finished)
	return nil
}

func (e *blockingExecutor) Disconnect(s *Session) error {
	return nil
}

func TestSessionServeWithPoll(t *testing.T) {
	setup := func(t *testing.T) (*Session, *websocket.Conn, *blockingExecutor) {
		conns := make(chan *websocket.Conn, 1)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			upgrader := websocket.Upgrader{}
			c, err := upgrader.Upgrade(w, r, nil)
			require.NoError(t, err)
			conns <- c
		}))
		t.Cleanup(server.Close)

		client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
		require.NoError(t, err)
		t.Cleanup(func() { client.Close() })

		executor := &blockingExecutor{
			started:  make(chan struct{}),
			release:  make(chan struct{}),
			finished: make(chan struct{}),
		}

		node := NewMockNode()
		session := BuildSession(ws.NewConnection(<-conns), common.NewSessionEnv("/cable-test", &map[string]string{}))
		session.executor = executor
		session.broker = node.broker
		session.uid = "1"
		session.Log = slog.With("sid", "1")
		session.readPool = utils.NewGoPool("read", 2)
		session.readWaitTimeout = time.Second

		return session, client, executor
	}

	t.Run("Resumes polling after handling a message", func(t *testing.T) {
		session, client, executor := setup(t)
		poller := &recordingPoller{}

		require.NoError(t, session.ServeWithPoll(poller, func() {}))
		require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte(`{"command":"subscribe","identifier":"test"}`)))

		poller.callback(netpoll.EventRead)
		<-executor.started
		close(executor.release)
		<-executor.finished

		require.Eventually(t, func() bool { return len(poller.Calls()) == 3 }, time.Second, time.Millisecond)
		assert.Equal(t, []string{"start", "watch", "resume"}, poller.Calls())
	})

	t.Run("Stops polling on close and doesn't resume after a message handled", func(t *testing.T) {
		session, client, executor := setup(t)
		poller := &recordingPoller{}

		require.NoError(t, session.ServeWithPoll(poller, func() {}))
		require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte(`{"command":"subscribe","identifier":"test"}`)))

		poller.callback(netpoll.EventRead)
		<-executor.started

		// The fd closes while the command is in progress; the OS can now reuse it
		session.conn.Close(ws.CloseNormalClosure, "")
		assert.Equal(t, []string{"start", "watch", "stop"}, poller.Calls())

		close(executor.release)
		<-executor.finished

		assert.Never(t, func() bool { return len(poller.Calls()) > 3 }, 100*time.Millisecond, time.Millisecond)
		assert.Equal(t, []string{"start", "watch", "stop"}, poller.Calls())
	})

	t.Run("Closes the session on hang-up while handling a message", func(t *testing.T) {
		session, client, executor := setup(t)
		session.connected.Store(true)
		poller := &recordingPoller{}

		require.NoError(t, session.ServeWithPoll(poller, func() {}))
		require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte(`{"command":"subscribe","identifier":"test"}`)))

		poller.callback(netpoll.EventRead)
		<-executor.started

		// The client leaves while the command waits
		poller.callback(netpoll.EventReadHup)

		assert.True(t, session.IsClosed())

		// The command is canceled by the session close, not released
		select {
		case <-executor.finished:
		case <-time.After(time.Second):
			t.Fatal("command is not canceled")
		}

		assert.Never(t, func() bool { return slices.Contains(poller.Calls(), "resume") }, 100*time.Millisecond, time.Millisecond)
		assert.Equal(t, []string{"start", "watch"}, poller.Calls()[:2])
		assert.NotContains(t, poller.Calls(), "resume")
	})

	t.Run("Doesn't start polling for a closed connection", func(t *testing.T) {
		session, _, _ := setup(t)
		poller := &recordingPoller{}

		session.conn.Close(ws.CloseNormalClosure, "")

		require.NoError(t, session.ServeWithPoll(poller, func() {}))
		assert.Empty(t, poller.Calls())
	})
}
