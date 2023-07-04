package hub

import (
	"context"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnsubscribe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gate := NewGate(ctx)

	session := NewMockSession("123")

	gate.Subscribe(session, "test", "test_channel")

	assert.Equal(t, 1, gate.Size())

	gate.Unsubscribe(session, "test", "test_channel")

	assert.Equal(t, 0, gate.Size())

	assert.Equal(t, 0, len(gate.streams))
	assert.Equal(t, 0, len(gate.sessionsStreams))
}

func TestShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	gate := NewGate(ctx)

	session := NewMockSession("123")

	gate.Subscribe(session, "test", "test_channel")

	gate.Broadcast(&common.StreamMessage{Stream: "test", Data: "1"})

	<-ctx.Done()

	gate.Broadcast(&common.StreamMessage{Stream: "test", Data: "2"})

	// Ignore first message if any, we want to make sure the second one is not received
	session.Read() // nolint:errcheck

	msg, err := session.Read()

	require.Error(t, err, "expected not to receive messages when context is done, but received: %s", msg)
}
