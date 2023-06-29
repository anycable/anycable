package lp

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mocks"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/ws"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHubFindSession(t *testing.T) {
	appNode := buildNode()
	conf := NewConfig()
	conf.FlushInterval = 10

	t.Run("when session is found", func(t *testing.T) {
		hub := NewHub(appNode, nil, &conf, slog.Default())
		w := httptest.NewRecorder()
		info := &server.RequestInfo{
			URL:     "http://localhost:3000/cable",
			Headers: &map[string]string{},
			UID:     "123",
		}

		id, session, err := hub.NewSession(w, info)
		require.NoError(t, err)
		require.NotNil(t, session)

		assert.Equal(t, 1, hub.Size())

		conn := session.UnderlyingConn().(*Connection)
		conn.Flush()

		<-conn.Context().Done()

		// second request
		w2 := httptest.NewRecorder()

		conn.Write([]byte("don't miss me"), time.Now().Add(1*time.Second)) // nolint: errcheck

		// wait a bit
		time.Sleep(10 * time.Millisecond)

		newSession, err := hub.FindSession(w2, id)
		require.NoError(t, err)
		require.NotNil(t, session)

		assert.Same(t, session, newSession)

		assert.Equal(t, 1, hub.Size())

		conn.Flush()
		<-conn.Context().Done()

		// We should see messages written before the second request
		require.Equal(t, "don't miss me\n", conn.testResponse())
	})

	t.Run("when session doesn't exist", func(t *testing.T) {
		hub := NewHub(appNode, nil, &conf, slog.Default())
		w := httptest.NewRecorder()
		session, err := hub.FindSession(w, "abc")

		require.NoError(t, err)
		require.Nil(t, session)

		assert.Equal(t, 0, hub.Size())

		// Wait for the flush
		time.Sleep(20 * time.Millisecond)

		// We should see the unauthorized message
		require.Equal(t, "{\"type\":\"disconnect\",\"reason\":\"session_expired\",\"reconnect\":true}", w.Body.String())
	})

	t.Run("when session is found but disconnected must respond with unauthorized", func(t *testing.T) {
		hub := NewHub(appNode, nil, &conf, slog.Default())
		w := httptest.NewRecorder()
		info := &server.RequestInfo{
			URL:     "http://localhost:3000/cable",
			Headers: &map[string]string{},
			UID:     "123",
		}

		id, session, err := hub.NewSession(w, info)
		require.NoError(t, err)
		require.NotNil(t, session)

		assert.Equal(t, 1, hub.Size())

		conn := session.UnderlyingConn().(*Connection)
		conn.Flush()
		<-conn.Context().Done()

		conn.Write([]byte("do you miss me?"), time.Now().Add(1*time.Second)) // nolint: errcheck

		session.Disconnect("test", ws.CloseNormalClosure)

		// wait a bit
		time.Sleep(10 * time.Millisecond)

		// second request
		w2 := httptest.NewRecorder()

		newSession, err := hub.FindSession(w2, id)
		require.NoError(t, err)
		require.Nil(t, newSession)

		assert.Equal(t, 0, hub.Size())

		// Wait for the flush
		time.Sleep(20 * time.Millisecond)

		// We should see the unauthorized message
		require.Equal(t, "{\"type\":\"disconnect\",\"reason\":\"session_expired\",\"reconnect\":true}", w2.Body.String())
	})
}

func TestNewSession(t *testing.T) {
	appNode := buildNode()
	conf := NewConfig()
	conf.FlushInterval = 10

	t.Run("when authentication passes", func(t *testing.T) {
		hub := NewHub(appNode, nil, &conf, slog.Default())
		w := httptest.NewRecorder()
		info := &server.RequestInfo{
			URL:     "/cable",
			Headers: &map[string]string{},
			UID:     "321",
		}

		id, session, err := hub.NewSession(w, info)

		require.NoError(t, err)
		require.NotNil(t, session)

		conn := session.UnderlyingConn().(*Connection)
		conn.Flush()

		assert.NotEmpty(t, id)
		assert.Equal(t, 1, hub.Size())

		<-conn.Context().Done()

		// We should see the unauthorized message
		require.Equal(t, "welcome\n", conn.testResponse())
	})

	t.Run("when authentication fails", func(t *testing.T) {
		hub := NewHub(appNode, nil, &conf, slog.Default())
		w := httptest.NewRecorder()
		info := &server.RequestInfo{
			URL:     "/failure",
			Headers: &map[string]string{},
			UID:     "321",
		}

		id, session, err := hub.NewSession(w, info)

		require.NoError(t, err)
		assert.NotNil(t, session)

		// We don not register failed sessions
		assert.Empty(t, id)
		assert.Equal(t, 0, hub.Size())

		conn := session.UnderlyingConn().(*Connection)

		<-conn.Context().Done()

		// We should see the unauthorized message
		require.Equal(t, "unauthorized\n", conn.testResponse())
	})
}

func TestHubReapingConnections(t *testing.T) {
	appNode := buildNode()
	conf := NewConfig()
	conf.FlushInterval = 10
	// deadlines have sec precision, lower values may result in flaky tests
	conf.KeepaliveTimeout = 1

	dconfig := node.NewDisconnectQueueConfig()
	dconfig.Rate = 1
	disconnector := node.NewDisconnectQueue(appNode, &dconfig, slog.Default())
	appNode.SetDisconnector(disconnector)

	hub := NewHub(appNode, nil, &conf, slog.Default())
	go hub.Run()
	defer hub.Shutdown(context.Background()) // nolint: errcheck

	w := httptest.NewRecorder()
	info := &server.RequestInfo{
		URL:     "http://localhost:3000/cable",
		Headers: &map[string]string{},
		UID:     "123",
	}

	staleID, session, err := hub.NewSession(w, info)
	require.NoError(t, err)
	require.NotNil(t, session)

	conn := session.UnderlyingConn().(*Connection)
	conn.Flush()

	w2 := httptest.NewRecorder()
	aliveID, aliveSession, err := hub.NewSession(w2, info)
	require.NoError(t, err)
	require.NotNil(t, aliveSession)

	aliveConn := session.UnderlyingConn().(*Connection)
	aliveConn.Flush()

	assert.Equal(t, 2, hub.Size())

	hub.Disconnected(staleID)
	hub.Disconnected(aliveID)

	// Wait a bit to update the last used time for alive session
	time.Sleep(500 * time.Millisecond)

	// Now let's try to fetch the second session again to update
	// its last used time
	w3 := httptest.NewRecorder()
	foundSession, err := hub.FindSession(w3, aliveID)

	require.NoError(t, err)
	require.NotNil(t, foundSession)

	hub.Disconnected(aliveID)

	aliveConn.Flush()

	// Wait for reaping to occur
	time.Sleep(600 * time.Millisecond)

	hub.reapStaleSessions()

	assert.Equal(t, 1, hub.Size())
	assert.Equal(t, 1, disconnector.Size())

	// Try to fetch the first session again -> should fail
	w4 := httptest.NewRecorder()
	newSession, err := hub.FindSession(w4, staleID)

	require.NoError(t, err)
	require.Nil(t, newSession)

	// Wait for another reaping to occur -> should reap aliveSession
	time.Sleep(1000 * time.Millisecond)

	hub.reapStaleSessions()

	assert.Equal(t, 0, hub.Size())
	assert.Equal(t, 2, disconnector.Size())
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

func buildNode() *node.Node {
	controller := mocks.NewMockController()
	config := node.NewConfig()
	config.HubGopoolSize = 2
	config.ReadGopoolSize = 2
	config.WriteGopoolSize = 2
	n := node.NewNode(&config, node.WithController(&controller), node.WithInstrumenter(metrics.NewMetrics(nil, 10, slog.Default())))
	n.SetBroker(broker.NewLegacyBroker(pubsub.NewLegacySubscriber(n)))
	n.SetDisconnector(&immediateDisconnector{n})
	return n
}
