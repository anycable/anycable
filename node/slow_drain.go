package node

import (
	"context"
	"time"
)

// slowDrainScheduler uses a leaky bucket-like algorithm to distribute disconnect calls
// over a given period of time.
// The bucket starts empty and is filled slowly over a given period of time until it is full.
// We start filling it slowly and then increase the rate of filling closer to the end of the period.
type slowDrainScheduler struct {
	ctx      context.Context
	interval time.Duration
	size     int
	deadline time.Time
	bucket   chan struct{}
}

var _ disconnectScheduler = (*slowDrainScheduler)(nil)

func newSlowDrainScheduler(ctx context.Context, size int, duration time.Duration, maxInterval int) *slowDrainScheduler {
	tick := time.Duration(duration.Nanoseconds() / int64(size))

	maxIntervalDuration := time.Duration(maxInterval) * time.Millisecond

	if tick > maxIntervalDuration {
		tick = maxIntervalDuration
	}

	deadline := time.Now().Add(duration)

	return &slowDrainScheduler{
		ctx:      ctx,
		interval: tick,
		size:     size,
		deadline: deadline,
		bucket:   make(chan struct{}, size),
	}
}

func (s *slowDrainScheduler) Start() {
	go func() {
		timer := time.NewTicker(s.interval)
		counter := 0

		defer timer.Stop()

		// Initial drop
		s.bucket <- struct{}{}

		for {
			select {
			case <-s.ctx.Done():
				return
			case <-timer.C:
				counter++
				if counter >= s.size {
					return
				}

				s.bucket <- struct{}{}
			}
		}
	}()
}

func (s *slowDrainScheduler) Continue() bool {
	select {
	case <-s.ctx.Done():
		return false
	case <-s.bucket:
		return true
	}
}
