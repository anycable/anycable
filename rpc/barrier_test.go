package rpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixedSizeBarrier_Saturated(t *testing.T) {
	barrier, err := NewFixedSizeBarrier(1)
	require.NoError(t, err)

	assert.False(t, barrier.Saturated())

	require.NoError(t, barrier.Acquire(context.Background()))

	// Busy, but no calls are waiting
	assert.False(t, barrier.Saturated())

	acquired := make(chan struct{})

	go func() {
		barrier.Acquire(context.Background()) // nolint:errcheck
		close(acquired)
	}()

	assert.Eventually(t, func() bool { return barrier.pendingNum.Load() == 1 }, time.Second, 10*time.Millisecond)

	// A call is waiting, but the last fast acquire was recent
	assert.False(t, barrier.Saturated())

	assert.Eventually(t, barrier.Saturated, 2*saturationInterval, 50*time.Millisecond)

	barrier.Release()
	<-acquired

	assert.False(t, barrier.Saturated())
}
