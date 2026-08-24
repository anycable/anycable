package redis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/rueidis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	redisAvailable = false
	redisURL       = os.Getenv("REDIS_URL")
)

// Check if Redis is available and skip tests otherwise
func init() {
	config := NewRedisConfig()

	if redisURL != "" {
		config.URL = redisURL
	}

	options, err := config.ToRueidisOptions()

	if err != nil {
		fmt.Printf("Failed to parse Redis URL: %v", err)
		return
	}

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

	c.Do(context.Background(), c.B().XgroupDestroy().Key("__test__").Group("__tg__").Build())
}

func TestStreamer(t *testing.T) {
	if !redisAvailable {
		t.Skip("Skipping Redis tests: no Redis available")
		return
	}

	rconfig := NewRedisConfig()

	if redisURL != "" {
		rconfig.URL = redisURL
	}

	stream := "__test__"
	group := "__tg__"
	block_ms := 500
	l := slog.Default()

	// Create a separate Redis client for test operations (publishing, waiting for consumers)
	// so we don't depend on the streamer's internal client lifecycle.
	options, err := rconfig.ToRueidisOptions()
	require.NoError(t, err)

	testClient, err := rueidis.NewClient(*options)
	require.NoError(t, err)
	defer testClient.Close()

	t.Run("Handles incoming messages", func(t *testing.T) {
		received := make(chan map[string]string, 10)

		handler := func(msg map[string]string) error {
			received <- msg
			return nil
		}

		streamer := NewStreamer(stream, group, &rconfig, l, StreamerWithHandler(handler), StreamerWithBlockMS(int64(block_ms)))

		err := streamer.Start()
		require.NoError(t, err)

		defer streamer.Shutdown(context.Background()) // nolint:errcheck

		<-streamer.Ready()

		require.NoError(t, publishToRedisStream(testClient, stream, "testo"))

		messages := drainStream(t, received, 1, 2*time.Second)

		msg := messages[0]

		assert.Equal(t, "testo", msg["payload"])
	})

	t.Run("With multiple subscribers", func(t *testing.T) {
		received := make(chan map[string]string, 10)

		handler := func(msg map[string]string) error {
			received <- msg
			return nil
		}

		streamer := NewStreamer(stream, group, &rconfig, l, StreamerWithHandler(handler), StreamerWithBlockMS(int64(block_ms)))

		err := streamer.Start()
		require.NoError(t, err)

		defer streamer.Shutdown(context.Background()) // nolint:errcheck

		streamer2 := NewStreamer(stream, group, &rconfig, l, StreamerWithHandler(handler), StreamerWithBlockMS(int64(block_ms)))
		err = streamer2.Start()
		require.NoError(t, err)

		defer streamer2.Shutdown(context.Background()) // nolint:errcheck

		<-streamer.Ready()
		<-streamer2.Ready()

		require.NoError(t, publishToRedisStream(testClient, stream, "123_test"))

		require.NoError(t, publishToRedisStream(testClient, stream, "124_test"))

		require.NoError(t, publishToRedisStream(testClient, stream, "125_test"))

		drainStream(t, received, 3, 2*time.Second)
	})
}

func TestStreamerAcksClaims(t *testing.T) {
	if !redisAvailable {
		t.Skip("Skipping Redis tests: no Redis available")
		return
	}

	rconfig := NewRedisConfig()

	if redisURL != "" {
		rconfig.URL = redisURL
	}

	stream := "__test__"
	group := "__tg__"
	block_ms := 100
	l := slog.Default()

	received := make(chan map[string]string, 10)
	closed := false

	var streamer *Streamer

	handler := func(msg map[string]string) error {
		received <- msg

		if msg["payload"] == "2" && !closed {
			closed = true
			// Close the connection to prevent consumer from ack-ing the message
			streamer.client.Close()
			streamer.reconnectAttempt = rconfig.MaxReconnectAttempts + 1
		}
		return nil
	}

	streamer = NewStreamer(stream, group, &rconfig, l, StreamerWithHandler(handler), StreamerWithBlockMS(int64(block_ms)))

	err := streamer.Start()
	require.NoError(t, err)
	defer streamer.Shutdown(context.Background()) // nolint:errcheck

	require.NoError(t, streamer.initClient())

	waitRedisStreamConsumers(t, streamer.client, 1)

	require.NoError(t, publishToRedisStream(streamer.client, stream, "1"))
	require.NoError(t, publishToRedisStream(streamer.client, stream, "2"))

	streamer2 := NewStreamer(stream, group, &rconfig, l, StreamerWithHandler(handler), StreamerWithBlockMS(int64(block_ms)))
	err = streamer2.Start()
	require.NoError(t, err)
	defer streamer2.Shutdown(context.Background()) // nolint:errcheck

	require.NoError(t, streamer2.initClient())

	waitRedisStreamConsumers(t, streamer2.client, 1)

	messages := drainStream(t, received, 3, 2*time.Second)

	assert.Equal(t, "1", messages[0]["payload"])
	assert.Equal(t, "2", messages[1]["payload"])
	// We haven't acked the last message within the first streamer,
	// so the second one must have picked it up
	assert.Equal(t, "2", messages[2]["payload"])
}

func drainStream[T any](t *testing.T, ch chan T, count int, timeout time.Duration) []T {
	buffer := make([]T, 0)

out:
	for {
		select {
		case msg := <-ch:
			buffer = append(buffer, msg)
			if len(buffer) == count {
				return buffer
			}
		case <-time.After(timeout):
			break out
		}
	}

	assert.Equalf(t, count, len(buffer), "haven't received %d messages on time", count)

	return buffer
}

func publishToRedisStream(client rueidis.Client, stream string, payload string) error {
	if client == nil {
		return errors.New("No Redis client configured")
	}

	res := client.Do(context.Background(),
		client.B().Xadd().Key(stream).Id("*").FieldValue().FieldValue("payload", payload).Build(),
	)

	return res.Error()
}

func waitRedisStreamConsumers(t *testing.T, client rueidis.Client, count int) {
	require.NotNil(t, client, "No Redis client configured")

	require.Eventuallyf(t,
		func() bool {
			res := client.Do(context.Background(), client.B().Arbitrary("client", "list").Build())
			clientsStr, err := res.ToString()
			if err != nil {
				return false
			}

			clients := strings.Split(clientsStr, "\n")
			readers := 0
			for _, clientMsg := range clients {
				if clientMsg == "" {
					continue
				}

				clientCmd := strings.Split(strings.Split(clientMsg, "cmd=")[1], " ")[0]

				if clientCmd == "xreadgroup" {
					readers++
				}
			}

			return readers >= count
		},
		3*time.Second,
		200*time.Millisecond,
		"No stream consumer were created",
	)
}
