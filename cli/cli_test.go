package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/anycable/anycable-go/broadcast"
	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/config"
	"github.com/anycable/anycable-go/mocks"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestRunnerDrainsBroadcastsBeforeBrokerShutdown(t *testing.T) {
	c := config.NewConfig()
	c.RPC.Implementation = "none"
	c.DisconnectorDisabled = true

	brokerClosed := false
	subscriberClosed := false
	serviceClosed := false
	delivered := []string{}
	service := mocks.NewBroadcaster(t)
	service.On("Shutdown", mock.Anything).Run(func(mock.Arguments) {
		serviceClosed = true
	}).Return(nil)
	subscriber := mocks.NewSubscriber(t)
	subscriber.On("Start", mock.Anything).Return(nil)
	subscriber.On("Broadcast", mock.Anything).Run(func(args mock.Arguments) {
		assert.False(t, subscriberClosed, "pub/sub must remain available while broadcasts drain")
		delivered = append(delivered, args.Get(0).(*common.StreamMessage).Data)
	})
	subscriber.On("BroadcastCommand", mock.Anything).Run(func(mock.Arguments) {
		assert.False(t, subscriberClosed, "pub/sub must remain available while the broker shuts down")
	})
	subscriber.On("Shutdown", mock.Anything).Run(func(mock.Arguments) {
		assert.False(t, serviceClosed, "shared services must outlive pub/sub")
		subscriberClosed = true
	}).Return(nil)

	appBroker := mocks.NewBroker(t)
	appBroker.On("Announce").Return("test broker")
	appBroker.On("Start", mock.Anything).Return(nil)
	appBroker.On("HandleBroadcast", mock.Anything).Return(func(msg *common.StreamMessage) error {
		assert.False(t, serviceClosed, "shared services must remain available while broadcasts drain")
		if brokerClosed {
			return errors.New("broker is closed")
		}
		subscriber.Broadcast(msg)
		return nil
	})
	appBroker.On("Shutdown", mock.Anything).Run(func(mock.Arguments) {
		assert.False(t, serviceClosed, "shared services must outlive the broker")
		brokerClosed = true
		subscriber.BroadcastCommand(&common.RemoteCommandMessage{})
	}).Return(nil)

	runner, err := NewRunner(&c, []Option{
		WithName("shutdown-test"),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithDefaultRPCController(),
		WithShutdownable(service),
		WithSubscriber(func(pubsub.Handler, *config.Config, *slog.Logger) (pubsub.Subscriber, error) {
			return subscriber, nil
		}),
		WithBroker(func(broker.Broadcaster, broker.Presenter, *config.Config, *slog.Logger) (broker.Broker, error) {
			return appBroker, nil
		}),
		WithBroadcasters(func(handler broadcast.Handler, _ *config.Config, _ *slog.Logger) ([]broadcast.Broadcaster, error) {
			broadcasters := []broadcast.Broadcaster{}
			for index := range 2 {
				broadcaster := mocks.NewBroadcaster(t)
				broadcaster.On("Start", mock.Anything).Return(nil)
				// A reader may finish an in-flight publication while Shutdown waits for it.
				payload := []byte(fmt.Sprintf(`{"stream":"shutdown-test","data":"%d"}`, index))
				broadcaster.On("Shutdown", mock.Anything).Return(func(context.Context) error {
					return handler.HandleBroadcast(payload)
				})
				broadcasters = append(broadcasters, broadcaster)
			}
			return broadcasters, nil
		}),
	})
	require.NoError(t, err)
	_, err = runner.runNode()
	require.NoError(t, err)

	for _, shutdownable := range runner.shutdownables {
		assert.NoError(t, shutdownable.Shutdown(context.Background()))
	}

	assert.Equal(t, []string{"0", "1"}, delivered)
	assert.True(t, brokerClosed)
	assert.True(t, subscriberClosed)
	assert.True(t, serviceClosed)
}

func TestCliConfig(t *testing.T) {
	_, err, _ := NewConfigFromCLI([]string{"-h"})
	require.NoError(t, err)
}

func TestCliConfigCustom(t *testing.T) {
	var custom string

	_, err, _ := NewConfigFromCLI(
		[]string{"", "-custom=foo"},
		WithCLICustomOptions(func() ([]cli.Flag, error) {
			flag := &cli.StringFlag{
				Name:        "custom",
				Destination: &custom,
				Value:       "bar",
				Required:    false,
			}
			return []cli.Flag{flag}, nil
		}),
	)

	require.NoError(t, err)
	assert.Equal(t, "foo", custom)
}
