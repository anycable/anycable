package pubsub

import (
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestHandler struct {
	messages chan (*common.StreamMessage)
	commands chan (*common.RemoteCommandMessage)
}

var _ Handler = (*TestHandler)(nil)

func NewTestHandler() *TestHandler {
	return &TestHandler{
		messages: make(chan *common.StreamMessage, 10),
		commands: make(chan *common.RemoteCommandMessage, 10),
	}
}

func (h *TestHandler) Broadcast(msg *common.StreamMessage) {
	h.messages <- msg
}

func (h *TestHandler) ExecuteRemoteCommand(cmd *common.RemoteCommandMessage) {
	h.commands <- cmd
}

func (h *TestHandler) Receive() *common.StreamMessage {
	timer := time.After(100 * time.Millisecond)

	select {
	case <-timer:
		return nil
	case msg := <-h.messages:
		return msg
	}
}

func (h *TestHandler) ReceiveCommand() *common.RemoteCommandMessage {
	timer := time.After(100 * time.Millisecond)

	select {
	case <-timer:
		return nil
	case msg := <-h.commands:
		return msg
	}
}

type subscriberFactory = func(handler *TestHandler) Subscriber
type subscriptionWaiter = func(subscriber Subscriber, stream string) error

func SharedSubscriberTests(t *testing.T, factory subscriberFactory, wait subscriptionWaiter) {
	handler := NewTestHandler()
	subscriber := factory(handler)
	done := make(chan error)

	err := subscriber.Start(done)
	require.NoError(t, err)

	require.NoError(t, wait(subscriber, "internal"))

	defer subscriber.Shutdown()

	t.Run("Broadcast", func(t *testing.T) {
		// Sbscribers may rely on known subscriptions
		subscriber.Subscribe("test")
		require.NoError(t, wait(subscriber, "test"))

		subscriber.Broadcast(&common.StreamMessage{Stream: "test", Data: "boo"})

		msg := handler.Receive()
		require.NotNil(t, msg)
		assert.Equal(t, "boo", msg.Data)
	})

	t.Run("Broadcast commands", func(t *testing.T) {
		subscriber.BroadcastCommand(&common.RemoteCommandMessage{Command: "test", Payload: []byte(`{"foo":"bar"}`)})

		cmd := handler.ReceiveCommand()
		require.NotNil(t, cmd)
		assert.Equal(t, "test", cmd.Command)
	})

	if !subscriber.IsMultiNode() {
		return
	}

	// Tests for multi-node subscribers require at least two handler and subscribers to
	// test re-transmission

	otherHandler := NewTestHandler()
	otherSubscriber := factory(otherHandler)

	err = otherSubscriber.Start(done)
	require.NoError(t, err)

	require.NoError(t, wait(otherSubscriber, "internal"))

	defer otherSubscriber.Shutdown()

	t.Run("Subscribe - Broadcast", func(t *testing.T) {
		subscriber.Subscribe("a")
		otherSubscriber.Subscribe("b")
		otherSubscriber.Subscribe("a")

		require.NoError(t, wait(subscriber, "a"))
		require.NoError(t, wait(otherSubscriber, "a"))
		require.NoError(t, wait(otherSubscriber, "b"))

		subscriber.Broadcast(&common.StreamMessage{Stream: "a", Data: "1"})

		msg := handler.Receive()
		require.NotNil(t, msg)
		assert.Equal(t, "1", msg.Data)
		assert.Equal(t, "a", msg.Stream)

		nextMsg := handler.Receive()
		assert.Nilf(t, nextMsg, "Must broadcast message once")

		msg = otherHandler.Receive()
		require.NotNil(t, msg)
		assert.Equal(t, "1", msg.Data)
		assert.Equal(t, "a", msg.Stream)

		nextMsg = otherHandler.Receive()
		assert.Nilf(t, nextMsg, "Must broadcast message once")

		subscriber.Broadcast(&common.StreamMessage{Stream: "b", Data: "2"})

		msg = handler.Receive()
		assert.Nilf(t, msg, "Should not broadcast message for unknown stream")

		msg = otherHandler.Receive()
		require.NotNil(t, msg)
		assert.Equal(t, "2", msg.Data)
		assert.Equal(t, "b", msg.Stream)
	})

	t.Run("Re-transmit commands", func(t *testing.T) {
		subscriber.BroadcastCommand(&common.RemoteCommandMessage{Command: "test"})

		cmd := handler.ReceiveCommand()
		require.NotNil(t, cmd)
		assert.Equal(t, "test", cmd.Command)

		cmd = otherHandler.ReceiveCommand()
		require.NotNil(t, cmd)
		assert.Equal(t, "test", cmd.Command)
	})

	t.Run("Subscribe - Broadcast - Unsubscribe - Broadcast", func(t *testing.T) {
		subscriber.Subscribe("a")
		otherSubscriber.Subscribe("a")

		require.NoError(t, wait(subscriber, "a"))
		require.NoError(t, wait(otherSubscriber, "a"))

		subscriber.Broadcast(&common.StreamMessage{Stream: "a", Data: "1"})

		msg := handler.Receive()
		require.NotNil(t, msg)
		assert.Equal(t, "1", msg.Data)
		assert.Equal(t, "a", msg.Stream)

		msg = otherHandler.Receive()
		require.NotNil(t, msg)
		assert.Equal(t, "1", msg.Data)
		assert.Equal(t, "a", msg.Stream)

		subscriber.Unsubscribe("a")
		require.NoError(t, wait(subscriber, "-a"))

		subscriber.Broadcast(&common.StreamMessage{Stream: "a", Data: "2"})

		msg = handler.Receive()
		assert.Nilf(t, msg, "Should not broadcast message for unsubscribed stream")

		msg = otherHandler.Receive()
		require.NotNil(t, msg)
		assert.Equal(t, "2", msg.Data)
		assert.Equal(t, "a", msg.Stream)
	})
}

func TestLegacySubscriber(t *testing.T) {
	SharedSubscriberTests(t, func(handler *TestHandler) Subscriber {
		return NewLegacySubscriber(handler)
	}, func(subscriber Subscriber, stream string) error { return nil })
}
