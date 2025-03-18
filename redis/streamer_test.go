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

	received := make(chan map[string]string, 10)

	handler := func(msg map[string]string) error {
		received <- msg
		return nil
	}

	t.Run("Handles incoming messages", func(t *testing.T) {
		streamer := NewStreamer(stream, group, &rconfig, l, StreamerWithHandler(handler), StreamerWithBlockMS(int64(block_ms)))

		err := streamer.Start()
		require.NoError(t, err)

		defer streamer.Shutdown(context.Background()) // nolint:errcheck

		require.NoError(t, streamer.initClient())

		require.NoError(t, waitRedisStreamConsumers(streamer.client, 1))

		require.NoError(t, publishToRedisStream(streamer.client, stream, "testo"))

		messages := drainStream(received)
		require.Equalf(t, 1, len(messages), "Expected 1 message, got %d", len(messages))

		msg := messages[0]

		assert.Equal(t, "testo", msg["payload"])
	})

	t.Run("With multiple subscribers", func(t *testing.T) {
		streamer := NewStreamer(stream, group, &rconfig, l, StreamerWithHandler(handler), StreamerWithBlockMS(int64(block_ms)))

		err := streamer.Start()
		require.NoError(t, err)

		defer streamer.Shutdown(context.Background()) // nolint:errcheck

		require.NoError(t, streamer.initClient())

		streamer2 := NewStreamer(stream, group, &rconfig, l, StreamerWithHandler(handler), StreamerWithBlockMS(int64(block_ms)))
		err = streamer2.Start()
		require.NoError(t, err)

		defer streamer2.Shutdown(context.Background()) // nolint:errcheck

		require.NoError(t, waitRedisStreamConsumers(streamer.client, 2))

		require.NoError(t, publishToRedisStream(streamer.client, stream, "123_test"))

		require.NoError(t, publishToRedisStream(streamer.client, stream, "124_test"))

		require.NoError(t, publishToRedisStream(streamer.client, stream, "125_test"))

		messages := drainStream(received)

		require.Equalf(t, 3, len(messages), "Expected 3 messages, got %d", len(messages))
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
	require.NoError(t, waitRedisStreamConsumers(streamer.client, 1))

	require.NoError(t, publishToRedisStream(streamer.client, stream, "1"))
	require.NoError(t, publishToRedisStream(streamer.client, stream, "2"))

	streamer2 := NewStreamer(stream, group, &rconfig, l, StreamerWithHandler(handler), StreamerWithBlockMS(int64(block_ms)))
	err = streamer2.Start()
	require.NoError(t, err)
	defer streamer2.Shutdown(context.Background()) // nolint:errcheck

	require.NoError(t, streamer2.initClient())
	require.NoError(t, waitRedisStreamConsumers(streamer2.client, 1))

	// We should wait for at least 2*blockTime to mark older consumer as stale
	// and claim its messages
	time.Sleep(300 * time.Millisecond)

	messages := drainStream(received)
	require.Equalf(t, 3, len(messages), "Expected 3 messages, got %d", len(messages))

	assert.Equal(t, "1", messages[0]["payload"])
	assert.Equal(t, "2", messages[1]["payload"])
	// We haven't acked the last message within the first streamer,
	// so the second one must have picked it up
	assert.Equal(t, "2", messages[2]["payload"])
}

func drainStream[T any](ch chan T) []T {
	buffer := make([]T, 0)

out:
	for {
		select {
		case msg := <-ch:
			buffer = append(buffer, msg)
		case <-time.After(time.Second):
			break out
		}
	}

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

func waitRedisStreamConsumers(client rueidis.Client, count int) error {
	if client == nil {
		return errors.New("No Redis client configured")
	}

	attempts := 0

	for {
		if attempts > 5 {
			return errors.New("No stream consumer were created")
		}

		res := client.Do(context.Background(), client.B().Arbitrary("client", "list").Build())
		clientsStr, err := res.ToString()

		if err == nil {
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

			if readers >= count {
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
		attempts++
	}
}
