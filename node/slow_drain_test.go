package node

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSlowDrainScheduler_Continue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	size := 2
	seconds := time.Second

	scheduler := newSlowDrainScheduler(ctx, size, seconds, 100)
	scheduler.Start()

	timer := time.After(time.Duration(5) * time.Second)
	// drainer
	var wg sync.WaitGroup
	wg.Add(size)

	go func() {
		for {
			if scheduler.Continue() {
				wg.Done()
			} else {
				return
			}
		}
	}()

	done := make(chan struct{})

	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-timer:
		t.Errorf("Scheduler should have filled the bucket")
	case <-done:
	}

	cancel()

	assert.False(t, scheduler.Continue())
}
