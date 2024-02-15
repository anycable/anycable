package pubsub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	rconfig "github.com/anycable/anycable-go/redis"
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
	config := rconfig.NewRedisConfig()

	if redisURL != "" {
		config.URL = redisURL
	}

	subscriber, err := NewRedisSubscriber(nil, &config, slog.Default())
	if err != nil {
		fmt.Printf("Failed to create redis subscriber: %v", err)
		return
	}

	err = subscriber.Start(make(chan error))

	if err != nil {
		fmt.Printf("Failed to start Redis subscriber: %v", err)
		return
	}

	err = subscriber.initClient()
	if err != nil {
		fmt.Printf("No Redis detected at %s: %v", config.URL, err)
		return
	}

	defer subscriber.Shutdown(context.Background()) // nolint:errcheck

	c := subscriber.client

	err = c.Do(context.Background(), c.B().Arbitrary("PING").Build()).Error()

	redisAvailable = err == nil
}

func TestRedisCommon(t *testing.T) {
	if !redisAvailable {
		t.Skip("Skipping Redis tests: no Redis available")
		return
	}

	config := rconfig.NewRedisConfig()

	if redisURL != "" {
		config.URL = redisURL
	}

	SharedSubscriberTests(t, func(handler *TestHandler) Subscriber {
		sub, err := NewRedisSubscriber(handler, &config, slog.Default())

		if err != nil {
			panic(err)
		}

		return sub
	}, waitRedisSubscription)
}

func TestRedisReconnect(t *testing.T) {
	if !redisAvailable {
		t.Skip("Skipping Redis tests: no Redis available")
		return
	}

	handler := NewTestHandler()
	config := rconfig.NewRedisConfig()

	if redisURL != "" {
		config.URL = redisURL
	}

	subscriber, err := NewRedisSubscriber(handler, &config, slog.Default())
	require.NoError(t, err)

	done := make(chan error)

	err = subscriber.Start(done)
	require.NoError(t, err)

	defer subscriber.Shutdown(context.Background()) // nolint:errcheck

	require.NoError(t, waitRedisSubscription(subscriber, "internal"))

	subscriber.Subscribe("reconnectos")
	require.NoError(t, waitRedisSubscription(subscriber, "reconnectos"))

	subscriber.Broadcast(&common.StreamMessage{Stream: "reconnectos", Data: "2022"})

	msg := handler.Receive()
	require.NotNil(t, msg)
	assert.Equal(t, "2022", msg.Data)

	// Drop Redis pus/sub connections
	require.NoError(t, dropRedisPubSubConnections(subscriber.client))
	require.NoError(t, waitRedisPubSubConnections(subscriber.client))

	require.NoError(t, waitRedisSubscription(subscriber, "reconnectos"))

	subscriber.Broadcast(&common.StreamMessage{Stream: "reconnectos", Data: "2023"})

	msg = handler.Receive()
	require.NotNil(t, msg)
	assert.Equal(t, "2023", msg.Data)
}

func waitRedisSubscription(subscriber Subscriber, stream string) error {
	s := subscriber.(*RedisSubscriber)

	if stream == "internal" {
		stream = s.config.InternalChannel
	}

	unsubscribing := false

	if strings.HasPrefix(stream, "-") {
		unsubscribing = true
		stream = strings.Replace(stream, "-", "", 1)
	}

	attempts := 0

	for {
		if attempts > 5 {
			if unsubscribing {
				return fmt.Errorf("Timeout exceeded to unsubscribe from stream: %s", stream)
			} else {
				return fmt.Errorf("Timeout exceeded to subscribe to stream: %s", stream)
			}
		}

		s.subMu.RLock()
		entry := s.subscriptionEntry(stream)
		state := subscriptionPending
		if entry != nil {
			state = entry.state
		}
		s.subMu.RUnlock()

		if unsubscribing {
			if entry == nil {
				return nil
			}
		} else {
			if entry == nil {
				return fmt.Errorf("No pending subscription: %s", stream)
			}

			if state == subscriptionCreated {
				return nil
			}
		}

		time.Sleep(100 * time.Millisecond)
		attempts++
	}
}

// Mimics Rails implementation: https://github.com/rails/rails/blob/6d581c43a77b8945df3d427273d357b67c303077/actioncable/test/subscription_adapter/redis_test.rb#L51-L67
func dropRedisPubSubConnections(client rueidis.Client) error {
	res := client.Do(context.Background(), client.B().Arbitrary("client", "kill", "type", "pubsub").Build())

	_, err := res.AsInt64()

	return err
}

func waitRedisPubSubConnections(client rueidis.Client) error {
	attempts := 0

	for {
		if attempts > 5 {
			return errors.New("No pub/sub connection were created")
		}

		res := client.Do(context.Background(), client.B().Arbitrary("pubsub", "channels").Build())
		channels, err := res.ToArray()

		if err == nil && len(channels) > 0 {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
		attempts++
	}
}
