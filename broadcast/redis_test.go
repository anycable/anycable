package broadcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
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

	c.Do(context.Background(), c.B().XgroupDestroy().Key("__anycable__").Group("bx").Build())
}

func TestRedisBroadcaster(t *testing.T) {
	if !redisAvailable {
		t.Skip("Skipping Redis tests: no Redis available")
		return
	}

	rconfig := rconfig.NewRedisConfig()
	config := NewRedisConfig()
	config.Redis = &rconfig

	if redisURL != "" {
		rconfig.URL = redisURL
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
		broadcaster := NewRedisBroadcaster(handler, &config, slog.Default())

		err := broadcaster.Start(errchan)
		require.NoError(t, err)

		defer broadcaster.Shutdown(context.Background()) // nolint:errcheck

		client := broadcaster.streamer.Client()

		require.NoError(t, waitRedisStreamConsumers(client, 1))

		require.NoError(t, publishToRedisStream(client, "__anycable__", string(payload)))

		messages := drainBroadcasts(broadcasts)
		require.Equalf(t, 1, len(messages), "Expected 1 message, got %d", len(messages))

		msg := messages[0]

		assert.Equal(t, "any_test", msg["stream"])
		assert.Equal(t, "123_test", msg["data"])
	})

	t.Run("With multiple subscribers", func(t *testing.T) {
		broadcaster := NewRedisBroadcaster(handler, &config, slog.Default())

		err := broadcaster.Start(errchan)
		require.NoError(t, err)

		defer broadcaster.Shutdown(context.Background()) // nolint:errcheck

		client := broadcaster.streamer.Client()

		broadcaster2 := NewRedisBroadcaster(handler, &config, slog.Default())
		err = broadcaster2.Start(errchan)
		require.NoError(t, err)

		defer broadcaster2.Shutdown(context.Background()) // nolint:errcheck

		require.NoError(t, waitRedisStreamConsumers(client, 2))

		require.NoError(t, publishToRedisStream(client, "__anycable__",
			string(utils.ToJSON(map[string]string{"stream": "any_test", "data": "123_test"})),
		))

		require.NoError(t, publishToRedisStream(client, "__anycable__",
			string(utils.ToJSON(map[string]string{"stream": "any_test", "data": "124_test"})),
		))

		require.NoError(t, publishToRedisStream(client, "__anycable__",
			string(utils.ToJSON(map[string]string{"stream": "any_test", "data": "125_test"})),
		))

		messages := drainBroadcasts(broadcasts)

		require.Equalf(t, 3, len(messages), "Expected 3 messages, got %d", len(messages))
	})
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

func TestRedisConfig__ToToml(t *testing.T) {
	config := NewRedisConfig()
	config.Stream = "test_stream"
	config.Group = "test_group"
	config.StreamReadBlockMilliseconds = 3000

	tomlStr := config.ToToml()

	assert.Contains(t, tomlStr, "stream = \"test_stream\"")
	assert.Contains(t, tomlStr, "group = \"test_group\"")
	assert.Contains(t, tomlStr, "stream_read_block_milliseconds = 3000")

	// Round-trip test
	config2 := NewRedisConfig()

	_, err := toml.Decode(tomlStr, &config2)
	require.NoError(t, err)

	assert.Equal(t, config, config2)
}
