package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDisconnectQueueShutdown(t *testing.T) {
	node := NewMockNode()

	session := NewMockSession("1")
	session2 := NewMockSession("2")

	dq := NewDisconnectQueue(&node, 1)

	dq.Enqueue(session)
	assert.Equal(t, 1, dq.Size(), "Disconnect queue size should be equal to 1")

	go func() {
		<-dq.shutdown
	}()

	go dq.Enqueue(session2)
	go dq.Enqueue(session2)
	go dq.Enqueue(session2)
	go dq.Enqueue(session2)
	go dq.Enqueue(session2)
	go dq.Enqueue(session2)

	dq.Shutdown()

	go dq.Enqueue(session2)
	go dq.Enqueue(session2)
	go dq.Enqueue(session2)
	go dq.Enqueue(session2)
	go dq.Enqueue(session2)
	go dq.Enqueue(session2)

	assert.Equal(t, 0, dq.Size(), "Disconnect queue size should be equal to 0")
}
