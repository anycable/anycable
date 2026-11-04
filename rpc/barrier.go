package rpc

import (
	"context"
	"fmt"
)

type Barrier interface {
	Acquire(ctx context.Context) error
	Release()
	BusyCount() int
	Capacity() int
	CapacityInfo() string
	Exhausted()
	HasDynamicCapacity() bool
	Start()
	Stop()
}

type FixedSizeBarrier struct {
	capacity     int
	capacityInfo string
	sem          chan (struct{})
}

var _ Barrier = (*FixedSizeBarrier)(nil)

func NewFixedSizeBarrier(capacity int) (*FixedSizeBarrier, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("RPC concurrency must be > 0")
	}

	sem := make(chan struct{}, capacity)

	for i := 0; i < capacity; i++ {
		sem <- struct{}{}
	}

	return &FixedSizeBarrier{
		capacity:     capacity,
		capacityInfo: fmt.Sprintf("%d", capacity),
		sem:          sem,
	}, nil
}

func (b *FixedSizeBarrier) Acquire(ctx context.Context) error {
	select {
	case <-b.sem:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *FixedSizeBarrier) Release() {
	b.sem <- struct{}{}
}

func (b *FixedSizeBarrier) BusyCount() int {
	// The number of in-flight request is the
	// the number of initial capacity "tickets" (concurrency)
	// minus the size of the semaphore channel
	return b.capacity - len(b.sem)
}

func (b *FixedSizeBarrier) Capacity() int {
	return b.capacity
}

func (b *FixedSizeBarrier) CapacityInfo() string {
	return b.capacityInfo
}

func (FixedSizeBarrier) Exhausted() {}

func (FixedSizeBarrier) HasDynamicCapacity() (res bool) { return }

func (FixedSizeBarrier) Start() {}

func (FixedSizeBarrier) Stop() {}
