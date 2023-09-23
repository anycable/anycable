package broker

import (
	"context"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/enats"
	natsconfig "github.com/anycable/anycable-go/nats"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNATSBroker_HistorySince_expiration(t *testing.T) {
	server := buildNATSServer()
	err := server.Start()
	require.NoError(t, err)
	defer server.Shutdown(context.Background()) // nolint:errcheck

	config := NewConfig()
	config.HistoryTTL = 1

	nconfig := natsconfig.NewNATSConfig()
	broker := NewNATSBroker(nil, &config, &nconfig)

	err = broker.Start()
	require.NoError(t, err)
	defer broker.Shutdown(context.Background()) // nolint: errcheck

	start := time.Now().Unix() - 10

	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "a"})
	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "b"})

	// Redis must expire the stream after 1 second
	time.Sleep(2 * time.Second)

	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "c"})
	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "d"})

	history, err := broker.HistorySince("test", start)
	require.NoError(t, err)

	assert.Len(t, history, 2)
	assert.EqualValues(t, 3, history[0].Offset)
	assert.Equal(t, "c", history[0].Data)
	assert.EqualValues(t, 4, history[1].Offset)
	assert.Equal(t, "d", history[1].Data)

	// Redis must expire the stream after 1 second
	time.Sleep(2 * time.Second)

	history, err = broker.HistorySince("test", start)
	require.NoError(t, err)
	assert.Nil(t, history)
}

func TestNATSBroker_HistorySince_with_limit(t *testing.T) {
	server := buildNATSServer()
	err := server.Start()
	require.NoError(t, err)
	defer server.Shutdown(context.Background()) // nolint:errcheck

	config := NewConfig()
	config.HistoryLimit = 2

	nconfig := natsconfig.NewNATSConfig()
	broker := NewNATSBroker(nil, &config, &nconfig)

	err = broker.Start()
	require.NoError(t, err)
	defer broker.Shutdown(context.Background()) // nolint: errcheck

	start := time.Now().Unix() - 10

	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "a"})
	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "b"})
	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "c"})

	history, err := broker.HistorySince("test", start)
	require.NoError(t, err)

	assert.Len(t, history, 2)
	assert.EqualValues(t, 3, history[1].Offset)
	assert.Equal(t, "c", history[1].Data)
}

func TestNATSBroker_HistoryFrom(t *testing.T) {
	server := buildNATSServer()
	err := server.Start()
	require.NoError(t, err)
	defer server.Shutdown(context.Background()) // nolint:errcheck

	config := NewConfig()

	nconfig := natsconfig.NewNATSConfig()
	broker := NewNATSBroker(nil, &config, &nconfig)

	err = broker.Start()
	require.NoError(t, err)
	defer broker.Shutdown(context.Background()) // nolint: errcheck

	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "a"})
	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "b"})
	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "c"})
	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "d"})
	broker.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "e"})

	t.Run("With current epoch", func(t *testing.T) {
		history, err := broker.HistoryFrom("test", broker.Epoch(), 2)
		require.NoError(t, err)

		assert.Len(t, history, 3)
		assert.EqualValues(t, 3, history[0].Offset)
		assert.Equal(t, "c", history[0].Data)
		assert.EqualValues(t, 4, history[1].Offset)
		assert.Equal(t, "d", history[1].Data)
		assert.EqualValues(t, 5, history[2].Offset)
		assert.Equal(t, "e", history[2].Data)
	})

	t.Run("When no new messages", func(t *testing.T) {
		history, err := broker.HistoryFrom("test", broker.Epoch(), 5)
		require.NoError(t, err)
		assert.Len(t, history, 0)
	})

	t.Run("When no stream", func(t *testing.T) {
		history, err := broker.HistoryFrom("unknown", broker.Epoch(), 2)
		require.Error(t, err)
		assert.Nil(t, history)
	})

	t.Run("With unknown epoch", func(t *testing.T) {
		history, err := broker.HistoryFrom("test", "unknown", 2)
		require.Error(t, err)
		require.Nil(t, history)
	})
}

type TestCacheable struct {
	data string
}

func (t *TestCacheable) ToCacheEntry() ([]byte, error) {
	return []byte(t.data), nil
}

func TestNATSBroker_Sessions(t *testing.T) {
	server := buildNATSServer()
	err := server.Start()
	require.NoError(t, err)
	defer server.Shutdown(context.Background()) // nolint:errcheck

	config := NewConfig()
	config.SessionsTTL = 1

	nconfig := natsconfig.NewNATSConfig()
	broker := NewNATSBroker(nil, &config, &nconfig)

	err = broker.Start()
	require.NoError(t, err)

	defer broker.Shutdown(context.Background()) // nolint: errcheck

	err = broker.CommitSession("test123", &TestCacheable{"cache-me"})
	require.NoError(t, err)

	anotherBroker := NewNATSBroker(nil, &config, &nconfig)
	anotherBroker.Start()                              // nolint: errcheck
	defer anotherBroker.Shutdown(context.Background()) // nolint: errcheck

	restored, err := anotherBroker.RestoreSession("test123")

	require.NoError(t, err)
	assert.Equalf(t, []byte("cache-me"), restored, "Expected to restore session data: %s", restored)

	// Expiration
	time.Sleep(2 * time.Second)

	expired, err := broker.RestoreSession("test123")
	require.NoError(t, err)
	assert.Nil(t, expired)

	err = broker.CommitSession("test345", &TestCacheable{"cache-me-again"})
	require.NoError(t, err)

	err = broker.FinishSession("test345")
	require.NoError(t, err)

	finished, err := anotherBroker.RestoreSession("test345")

	require.NoError(t, err)
	assert.Equal(t, []byte("cache-me-again"), finished)

	// Expiration
	time.Sleep(2 * time.Second)

	finishedStale, err := anotherBroker.RestoreSession("test345")
	require.NoError(t, err)
	assert.Nil(t, finishedStale)
}

func TestNATSBroker_SessionsTTLChange(t *testing.T) {
	server := buildNATSServer()
	err := server.Start()
	require.NoError(t, err)
	defer server.Shutdown(context.Background()) // nolint:errcheck

	config := NewConfig()
	config.SessionsTTL = 1

	nconfig := natsconfig.NewNATSConfig()
	broker := NewNATSBroker(nil, &config, &nconfig)

	err = broker.Start()
	require.NoError(t, err)

	defer broker.Shutdown(context.Background()) // nolint: errcheck

	err = broker.CommitSession("test123", &TestCacheable{"cache-me"})
	require.NoError(t, err)

	aConfig := NewConfig()
	aConfig.SessionsTTL = 2

	anotherBroker := NewNATSBroker(nil, &aConfig, &nconfig)
	anotherBroker.Start()                              // nolint: errcheck
	defer anotherBroker.Shutdown(context.Background()) // nolint: errcheck

	// The session must be missing since we recreated the bucket due to TTL change
	missing, err := anotherBroker.RestoreSession("test123")

	require.NoError(t, err)
	assert.Nil(t, missing)

	err = anotherBroker.CommitSession("test234", &TestCacheable{"cache-me-again"})
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	// Shouldn't fail and catch up a new bucket
	restored, err := broker.RestoreSession("test234")
	require.NoError(t, err)
	assert.Equalf(t, []byte("cache-me-again"), restored, "Expected to restore session data: %s", restored)

	time.Sleep(2 * time.Second)

	expired, err := broker.RestoreSession("test234")
	require.NoError(t, err)
	assert.Nil(t, expired)
}

func TestNATSBroker_Epoch(t *testing.T) {
	server := buildNATSServer()
	err := server.Start()
	require.NoError(t, err)
	defer server.Shutdown(context.Background()) // nolint:errcheck

	config := NewConfig()

	nconfig := natsconfig.NewNATSConfig()
	broker := NewNATSBroker(nil, &config, &nconfig)

	err = broker.Start()
	require.NoError(t, err)
	defer broker.Shutdown(context.Background()) // nolint: errcheck

	epoch := broker.Epoch()

	anotherBroker := NewNATSBroker(nil, &config, &nconfig)
	anotherBroker.Start()                              // nolint: errcheck
	defer anotherBroker.Shutdown(context.Background()) // nolint: errcheck

	assert.Equal(t, epoch, anotherBroker.Epoch())
}

func buildNATSServer() *enats.Service {
	conf := enats.NewConfig()
	conf.JetStream = true
	service := enats.NewService(&conf)

	return service
}
