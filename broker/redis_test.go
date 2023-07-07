package broker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/pubsub"
	rconfig "github.com/anycable/anycable-go/redis"
	"github.com/redis/rueidis"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	redisAvailable = false
	redisURL       = os.Getenv("REDIS_URL")
	redisConfig    = rconfig.NewRedisConfig()
)

// Check if Redis is available and skip tests otherwise
func init() {
	if redisURL != "" {
		redisConfig.URL = redisURL
	}

	options, _ := redisConfig.ToRueidisOptions() // nolint: errcheck

	c, err := rueidis.NewClient(*options)

	if err != nil {
		fmt.Printf("Failed to connect to Redis: %v", err)
		return
	}

	err = c.Do(context.Background(), c.B().Arbitrary("PING").Build()).Error()

	redisAvailable = err == nil

	if !redisAvailable {
		return
	}
}

func TestRedisBroker_HistorySince_expiration(t *testing.T) {
	if !redisAvailable {
		t.Skip("Redis is not available")
		return
	}

	config := NewConfig()
	config.HistoryTTL = 1

	broadcastHandler := FakeBroadastHandler{}
	broadcaster := pubsub.NewLegacySubscriber(broadcastHandler)

	broker := NewRedisBroker(broadcaster, &config, &redisConfig, slog.Default())
	err := broker.Start(nil)
	require.NoError(t, err)
	defer broker.Shutdown(context.Background()) // nolint: errcheck
	require.NoError(t, broker.Reset())

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

func TestRedisBroker_HistorySince_with_limit(t *testing.T) {
	if !redisAvailable {
		t.Skip("Redis is not available")
		return
	}

	config := NewConfig()
	config.HistoryLimit = 2

	broadcastHandler := FakeBroadastHandler{}
	broadcaster := pubsub.NewLegacySubscriber(broadcastHandler)

	broker := NewRedisBroker(broadcaster, &config, &redisConfig, slog.Default())
	err := broker.Start(nil)
	require.NoError(t, err)
	defer broker.Shutdown(context.Background()) // nolint: errcheck
	require.NoError(t, broker.Reset())

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

func TestRedisBroker_HistoryFrom(t *testing.T) {
	if !redisAvailable {
		t.Skip("Redis is not available")
		return
	}

	config := NewConfig()

	broadcastHandler := FakeBroadastHandler{}
	broadcaster := pubsub.NewLegacySubscriber(broadcastHandler)

	broker := NewRedisBroker(broadcaster, &config, &redisConfig, slog.Default())
	err := broker.Start(nil)
	require.NoError(t, err)
	defer broker.Shutdown(context.Background()) // nolint: errcheck
	require.NoError(t, broker.Reset())

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

func TestRedisBroker_Sessions(t *testing.T) {
	if !redisAvailable {
		t.Skip("Redis is not available")
		return
	}

	config := NewConfig()
	config.SessionsTTL = 1

	broadcastHandler := FakeBroadastHandler{}
	broadcaster := pubsub.NewLegacySubscriber(broadcastHandler)

	broker := NewRedisBroker(broadcaster, &config, &redisConfig, slog.Default())
	err := broker.Start(nil)
	require.NoError(t, err)

	defer broker.Shutdown(context.Background()) // nolint: errcheck

	require.NoError(t, broker.Reset())

	err = broker.CommitSession("test123", &TestCacheable{"cache-me"})
	require.NoError(t, err)

	anotherBroker := NewRedisBroker(broadcaster, &config, &redisConfig, slog.Default())
	anotherBroker.Start(nil)                           // nolint: errcheck
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

func TestRedisBroker_Epoch(t *testing.T) {
	if !redisAvailable {
		t.Skip("Redis is not available")
		return
	}

	config := NewConfig()

	broker := NewRedisBroker(nil, &config, &redisConfig, slog.Default())
	err := broker.Start(nil)
	require.NoError(t, err)
	defer broker.Shutdown(context.Background()) // nolint: errcheck

	require.NoError(t, broker.Reset())

	epoch := broker.Epoch()

	broadcastHandler := FakeBroadastHandler{}
	broadcaster := pubsub.NewLegacySubscriber(broadcastHandler)

	anotherBroker := NewRedisBroker(broadcaster, &config, &redisConfig, slog.Default())
	anotherBroker.Start(nil)                           // nolint: errcheck
	defer anotherBroker.Shutdown(context.Background()) // nolint: errcheck

	assert.Equal(t, epoch, anotherBroker.Epoch())

	err = broker.client.Do(context.Background(), broker.client.B().Del().Key("$ac:e").Build()).Error()
	require.NoError(t, err)

	oneMoreBroker := NewRedisBroker(broadcaster, &config, &redisConfig, slog.Default())
	oneMoreBroker.Start(nil)                           // nolint: errcheck
	defer oneMoreBroker.Shutdown(context.Background()) // nolint: errcheck

	assert.NotEqual(t, epoch, oneMoreBroker.Epoch())
}
