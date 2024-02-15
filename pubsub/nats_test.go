package pubsub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/enats"
	"github.com/anycable/anycable-go/nats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	nats_server "github.com/nats-io/nats.go"
)

func TestNATSCommon(t *testing.T) {
	server := buildNATSServer()
	err := server.Start()
	require.NoError(t, err)
	defer server.Shutdown(context.Background()) // nolint:errcheck

	config := nats.NewNATSConfig()

	SharedSubscriberTests(t, func(handler *TestHandler) Subscriber {
		sub, err := NewNATSSubscriber(handler, &config, slog.Default())

		if err != nil {
			panic(err)
		}

		return sub
	}, waitNATSSubscription)
}

func TestNATSReconnect(t *testing.T) {
	server := buildNATSServer()
	err := server.Start()
	require.NoError(t, err)
	defer server.Shutdown(context.Background()) // nolint:errcheck

	handler := NewTestHandler()
	config := nats.NewNATSConfig()

	subscriber, err := NewNATSSubscriber(handler, &config, slog.Default())
	require.NoError(t, err)

	done := make(chan error)

	err = subscriber.Start(done)
	require.NoError(t, err)

	defer subscriber.Shutdown(context.Background()) // nolint:errcheck

	require.NoError(t, waitNATSSubscription(subscriber, "internal"))

	subscriber.Subscribe("reconnectos")
	require.NoError(t, waitNATSSubscription(subscriber, "reconnectos"))

	subscriber.Broadcast(&common.StreamMessage{Stream: "reconnectos", Data: "2023"})

	msg := handler.Receive()
	require.NotNil(t, msg)
	assert.Equal(t, "2023", msg.Data)

	// Reload NATS server
	err = server.Shutdown(context.Background())
	require.NoError(t, err)
	err = server.Start()
	require.NoError(t, err)

	err = waitNATSConnectionActive(subscriber)
	require.NoError(t, err)

	subscriber.Broadcast(&common.StreamMessage{Stream: "reconnectos", Data: "2023"})

	msg = handler.Receive()
	require.NotNil(t, msg)
	assert.Equal(t, "2023", msg.Data)
}

func waitNATSSubscription(subscriber Subscriber, stream string) error {
	s := subscriber.(*NATSSubscriber)

	err := waitNATSConnectionActive(s)

	if err != nil {
		return err
	}

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
		sub := s.subscriptions[stream]
		s.subMu.RUnlock()

		if unsubscribing {
			if sub == nil {
				return nil
			}
		} else {
			if sub == nil {
				return fmt.Errorf("No pending subscription: %s", stream)
			}

			// We cannot get the subscription's status, so let's add a bit of delay here
			time.Sleep(100 * time.Millisecond)

			return nil
		}

		time.Sleep(100 * time.Millisecond)
		attempts++
	}
}

func waitNATSConnectionActive(s *NATSSubscriber) error {
	attempts := 0

	for {
		if attempts > 5 {
			return errors.New("Connection wasn't restored")
		}

		if s.conn.Status() == nats_server.CONNECTED {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
		attempts++
	}
}

func buildNATSServer() *enats.Service {
	conf := enats.NewConfig()
	service := enats.NewService(&conf, slog.Default())

	return service
}
