package broadcast

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
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

func TestLegacyRedisConfig__ToToml(t *testing.T) {
	conf := NewLegacyRedisConfig()
	conf.Channel = "_test_"

	tomlStr := conf.ToToml()

	assert.Contains(t, tomlStr, "channel = \"_test_\"")

	// Round-trip test
	conf2 := NewLegacyRedisConfig()

	_, err := toml.Decode(tomlStr, &conf2)
	require.NoError(t, err)

	assert.Equal(t, conf, conf2)
}

func TestLegacyRedisBroadcaster(t *testing.T) {
	if !redisAvailable {
		t.Skip("Skipping Redis tests: no Redis available")
		return
	}

	rconf := rconfig.NewRedisConfig()
	config := NewLegacyRedisConfig()
	config.Redis = &rconf

	if redisURL != "" {
		rconf.URL = redisURL
	}

	handler := &mocks.Handler{}
	errchan := make(chan error, 1)
	broadcasts := make(chan map[string]string, 10)

	handler.On(
		"HandlePubSub",
		mock.Anything,
	).Run(func(args mock.Arguments) {
		data := args.Get(0).([]byte)
		var msg map[string]string
		json.Unmarshal(data, &msg) // nolint: errcheck

		broadcasts <- msg
	}).Return(nil)

	t.Run("Handles pubsub broadcasts", func(t *testing.T) {
		broadcaster := NewLegacyRedisBroadcaster(handler, &config, slog.Default())

		err := broadcaster.Start(errchan)
		require.NoError(t, err)

		defer broadcaster.Shutdown(context.Background()) // nolint:errcheck

		client, err := createRedisClient(&rconf)
		require.NoError(t, err)
		defer client.Close()

		// Wait for subscription to be established
		require.NoError(t, waitLegacyRedisPubSubSubscription(client, config.Channel))

		payload := utils.ToJSON(map[string]string{"stream": "legacy_test", "data": "456_test"})
		require.NoError(t, publishToRedisPubSub(client, config.Channel, string(payload)))

		messages := drainBroadcasts(broadcasts)
		require.Equalf(t, 1, len(messages), "Expected 1 message, got %d", len(messages))

		msg := messages[0]

		assert.Equal(t, "legacy_test", msg["stream"])
		assert.Equal(t, "456_test", msg["data"])
	})
}

func TestLegacyRedisBroadcasterReconnect(t *testing.T) {
	if !redisAvailable {
		t.Skip("Skipping Redis tests: no Redis available")
		return
	}

	rconf := rconfig.NewRedisConfig()
	config := NewLegacyRedisConfig()
	config.Redis = &rconf
	config.Channel = "__anycable_legacy_reconnect_test__"

	if redisURL != "" {
		rconf.URL = redisURL
	}

	handler := &mocks.Handler{}
	errchan := make(chan error, 1)
	broadcasts := make(chan map[string]string, 10)

	handler.On(
		"HandlePubSub",
		mock.Anything,
	).Run(func(args mock.Arguments) {
		data := args.Get(0).([]byte)
		var msg map[string]string
		json.Unmarshal(data, &msg) // nolint: errcheck

		broadcasts <- msg
	}).Return(nil)

	broadcaster := NewLegacyRedisBroadcaster(handler, &config, slog.Default())

	err := broadcaster.Start(errchan)
	require.NoError(t, err)

	defer broadcaster.Shutdown(context.Background()) // nolint:errcheck

	client, err := createRedisClient(&rconf)
	require.NoError(t, err)
	defer client.Close()

	// Wait for subscription to be established
	require.NoError(t, waitLegacyRedisPubSubSubscription(client, config.Channel))

	// Send first message
	payload := utils.ToJSON(map[string]string{"stream": "reconnect_test", "data": "before_disconnect"})
	require.NoError(t, publishToRedisPubSub(client, config.Channel, string(payload)))

	messages := drainBroadcasts(broadcasts)
	require.Equalf(t, 1, len(messages), "Expected 1 message before disconnect, got %d", len(messages))
	assert.Equal(t, "before_disconnect", messages[0]["data"])

	// Drop Redis pub/sub connections (mimics connection failure)
	require.NoError(t, dropRedisPubSubConnections(client))
	require.NoError(t, waitRedisPubSubConnectionsRestored(client))

	// Wait for subscription to be re-established
	require.NoError(t, waitLegacyRedisPubSubSubscription(client, config.Channel))

	// Send message after reconnection
	payload = utils.ToJSON(map[string]string{"stream": "reconnect_test", "data": "after_reconnect"})
	require.NoError(t, publishToRedisPubSub(client, config.Channel, string(payload)))

	messages = drainBroadcasts(broadcasts)
	require.Equalf(t, 1, len(messages), "Expected 1 message after reconnect, got %d", len(messages))
	assert.Equal(t, "after_reconnect", messages[0]["data"])
}

func createRedisClient(config *rconfig.RedisConfig) (rueidis.Client, error) {
	options, err := config.ToRueidisOptions()
	if err != nil {
		return nil, err
	}

	return rueidis.NewClient(*options)
}

func publishToRedisPubSub(client rueidis.Client, channel string, payload string) error {
	res := client.Do(context.Background(),
		client.B().Publish().Channel(channel).Message(payload).Build(),
	)
	return res.Error()
}

func waitLegacyRedisPubSubSubscription(client rueidis.Client, channel string) error {
	attempts := 0

	for {
		if attempts > 10 {
			return errors.New("Timeout waiting for pubsub subscription")
		}

		res := client.Do(context.Background(), client.B().PubsubChannels().Build())
		channels, err := res.AsStrSlice()

		if err == nil {
			for _, ch := range channels {
				if ch == channel {
					return nil
				}
			}
		}

		time.Sleep(200 * time.Millisecond)
		attempts++
	}
}

// Mimics Rails implementation: https://github.com/rails/rails/blob/6d581c43a77b8945df3d427273d357b67c303077/actioncable/test/subscription_adapter/redis_test.rb#L51-L67
func dropRedisPubSubConnections(client rueidis.Client) error {
	res := client.Do(context.Background(), client.B().Arbitrary("client", "kill", "type", "pubsub").Build())

	_, err := res.AsInt64()

	return err
}

func waitRedisPubSubConnectionsRestored(client rueidis.Client) error {
	attempts := 0

	for {
		if attempts > 10 {
			return errors.New("No pub/sub connections were restored")
		}

		res := client.Do(context.Background(), client.B().PubsubChannels().Build())
		channels, err := res.AsStrSlice()

		if err == nil && len(channels) > 0 {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
		attempts++
	}
}
