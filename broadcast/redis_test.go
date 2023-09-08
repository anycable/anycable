package broadcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/anycable/anycable-go/mocks"
	rconfig "github.com/anycable/anycable-go/redis"
	"github.com/anycable/anycable-go/utils"
	"github.com/redis/rueidis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	redisAvailable = false
	redisURL       = os.Getenv("REDIS_URL")
)

// Check if Redis is available and skip tests otherwise
func init() {
	config := rconfig.NewRedisConfig()

	if redisURL != "" {
		config.URL = redisURL
	}

	options, err := config.ToRueidisOptions()

	if err != nil {
		fmt.Printf("Failed to create Redis URL: %v", err)
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

	c.Do(context.Background(), c.B().XgroupDestroy().Key("__anycable__").Group("bx").Build())
}

func TestRedisBroadcaster(t *testing.T) {
	if !redisAvailable {
		t.Skip("Skipping Redis tests: no Redis available")
		return
	}

	config := rconfig.NewRedisConfig()

	if redisURL != "" {
		config.URL = redisURL
	}

	config.StreamReadBlockMilliseconds = 500

	handler := &mocks.Handler{}
	errchan := make(chan error)
	broadcasts := make(chan map[string]string, 10)

	payload := utils.ToJSON(map[string]string{"stream": "any_test", "data": "123_test"})

	handler.On(
		"HandleBroadcast",
		mock.Anything,
	).Run(func(args mock.Arguments) {
		data := args.Get(0).([]byte)
		var msg map[string]string
		json.Unmarshal(data, &msg) // nolint: errcheck

		broadcasts <- msg
	})

	t.Run("Handles broadcasts", func(t *testing.T) {
		broadcaster := NewRedisBroadcaster(handler, &config)

		err := broadcaster.Start(errchan)
		require.NoError(t, err)

		defer broadcaster.Shutdown(context.Background()) // nolint:errcheck

		require.NoError(t, broadcaster.initClient())

		require.NoError(t, waitRedisStreamConsumers(broadcaster.client, 1))

		require.NoError(t, publishToRedisStream(broadcaster.client, "__anycable__", string(payload)))

		messages := drainBroadcasts(broadcasts)
		require.Equalf(t, 1, len(messages), "Expected 1 message, got %d", len(messages))

		msg := messages[0]

		assert.Equal(t, "any_test", msg["stream"])
		assert.Equal(t, "123_test", msg["data"])
	})

	t.Run("With multiple subscribers", func(t *testing.T) {
		broadcaster := NewRedisBroadcaster(handler, &config)

		err := broadcaster.Start(errchan)
		require.NoError(t, err)

		defer broadcaster.Shutdown(context.Background()) // nolint:errcheck

		require.NoError(t, broadcaster.initClient())

		broadcaster2 := NewRedisBroadcaster(handler, &config)
		err = broadcaster2.Start(errchan)
		require.NoError(t, err)

		defer broadcaster2.Shutdown(context.Background()) // nolint:errcheck

		require.NoError(t, waitRedisStreamConsumers(broadcaster.client, 2))

		require.NoError(t, publishToRedisStream(broadcaster.client, "__anycable__",
			string(utils.ToJSON(map[string]string{"stream": "any_test", "data": "123_test"})),
		))

		require.NoError(t, publishToRedisStream(broadcaster.client, "__anycable__",
			string(utils.ToJSON(map[string]string{"stream": "any_test", "data": "124_test"})),
		))

		require.NoError(t, publishToRedisStream(broadcaster.client, "__anycable__",
			string(utils.ToJSON(map[string]string{"stream": "any_test", "data": "125_test"})),
		))

		messages := drainBroadcasts(broadcasts)

		require.Equalf(t, 3, len(messages), "Expected 3 messages, got %d", len(messages))
	})
}

func TestRedisBroadcasterAcksClaims(t *testing.T) {
	if !redisAvailable {
		t.Skip("Skipping Redis tests: no Redis available")
		return
	}

	config := rconfig.NewRedisConfig()
	// Make it short to avoid sleeping for too long in tests
	config.StreamReadBlockMilliseconds = 100

	if redisURL != "" {
		config.URL = redisURL
	}

	handler := &mocks.Handler{}
	broadcaster := NewRedisBroadcaster(handler, &config)

	errchan := make(chan error)
	broadcasts := make(chan string, 10)

	closed := false

	handler.On(
		"HandleBroadcast",
		mock.Anything,
	).Run(func(args mock.Arguments) {
		msg := string(args.Get(0).([]byte))
		broadcasts <- msg

		if msg == "2" && !closed {
			closed = true
			// Close the connection to prevent consumer from ack-ing the message
			broadcaster.client.Close()
			broadcaster.reconnectAttempt = config.MaxReconnectAttempts + 1
		}
	})

	err := broadcaster.Start(errchan)
	require.NoError(t, err)
	defer broadcaster.Shutdown(context.Background()) // nolint:errcheck

	require.NoError(t, broadcaster.initClient())
	require.NoError(t, waitRedisStreamConsumers(broadcaster.client, 1))

	require.NoError(t, publishToRedisStream(broadcaster.client, "__anycable__", "1"))
	require.NoError(t, publishToRedisStream(broadcaster.client, "__anycable__", "2"))

	broadcaster2 := NewRedisBroadcaster(handler, &config)
	err = broadcaster2.Start(errchan)
	require.NoError(t, err)
	defer broadcaster2.Shutdown(context.Background()) // nolint:errcheck

	require.NoError(t, broadcaster2.initClient())
	require.NoError(t, waitRedisStreamConsumers(broadcaster2.client, 1))

	// We should wait for at least 2*blockTime to mark older consumer as stale
	// and claim its messages
	time.Sleep(300 * time.Millisecond)

	messages := drainBroadcasts(broadcasts)
	require.Equalf(t, 3, len(messages), "Expected 3 messages, got %d", len(messages))

	assert.Equal(t, "1", messages[0])
	assert.Equal(t, "2", messages[1])
	// We haven't acked the last message within the first broadcaster,
	// so the second one must have picked it up
	assert.Equal(t, "2", messages[1])
}

func drainBroadcasts[T any](ch chan T) []T {
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
