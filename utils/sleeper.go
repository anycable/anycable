package utils

import (
	"context"
	"time"
)

type Sleeper struct {
	delay time.Duration
}

// NewSleeper returns a Shutdownable which sleeps on Shutdown()
func NewSleeper(delay time.Duration) Sleeper {
	return Sleeper{delay}
}

func (s Sleeper) Shutdown(ctx context.Context) error {
	select {
	case <-ctx.Done():
	case <-time.After(s.delay):
	}

	return nil
}
