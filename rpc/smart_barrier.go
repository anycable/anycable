package rpc

import (
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

type SmartBarrierConfig struct {
	// Initial concurrency value
	InitialConcurrency int
	// Min value for concurrency
	MinConcurrency int
	// Max value for concurrency
	MaxConcurrency int
	// GrowthFactor is a coefficient we use to scale the capacity up or down
	GrowthFactor float64
	// PendingFactor is the ratio between current pending tasks and
	// the current capacity which should be considered high enough to try to grow
	PendingFactor float64
	// ExhaustedFactor is the ratio between exhausted tasks to the total number of tasks
	// since the last tick which should be considered high enough to try to shrink
	ExhaustedFactor float64
	// TickInterval defines how often to invalidate the current state (in seconds)
	TickInterval int
}

// SmartBarrier is an RPC barrier, which automatically scales up and down
// the concurrency limit depending on pending/exhausted requests.
//
// It's logic could be seen as a finite automaton. It's input is a vector of two bool elements, (grow, shrink).
//
// grow = (pending / total) > pending_factor
// shrink = (exhausted / total) > exhausted_factor
//
// states/inputs    | (0, 0) | (0, 1)  | (1, 0)  | (1, 1)
// -----------------|--------|---------|---------|----------
// idle (i)         |   i    |   s     |   g     |   s
// shrinking (s)    |   i    |   s     |   i     |   s
// growing (g)      |   i    |   i     |   g     |   s
type SmartBarrier struct {
	capacityInfo string
	capacity     int
	sem          chan (struct{})

	callsCount     atomic.Uint64
	exhaustedCount atomic.Uint64
	pendingNum     atomic.Int64

	tickTimer    *time.Timer
	tickInterval time.Duration

	// Mutex to update capacity and state
	mu sync.RWMutex

	// 0 - idle, 1 - growing, -1 — shrinking
	state int

	config *SmartBarrierConfig
	log    *slog.Logger
}

func NewSmartBarrierConfig() SmartBarrierConfig {
	return SmartBarrierConfig{
		InitialConcurrency: 25,
		MaxConcurrency:     100,
		MinConcurrency:     5,
		GrowthFactor:       1.1,
		PendingFactor:      3,
		ExhaustedFactor:    0.05,
		TickInterval:       10,
	}
}

var _ Barrier = (*SmartBarrier)(nil)

func NewSmartBarrier(config SmartBarrierConfig, l *slog.Logger) *SmartBarrier {
	capacity := config.InitialConcurrency

	if capacity > config.MaxConcurrency {
		l.Warn(fmt.Sprintf("initial concurrency (%d) is greater than max concurrency (%d), adjusting", capacity, config.MaxConcurrency))
		capacity = config.MaxConcurrency
	}

	if capacity < config.MinConcurrency {
		l.Warn(fmt.Sprintf("initial concurrency (%d) is less than min concurrency (%d), adjusting", capacity, config.MinConcurrency))
		capacity = config.MinConcurrency
	}

	sem := make(chan struct{}, config.MaxConcurrency)

	for i := 0; i < capacity; i++ {
		sem <- struct{}{}
	}

	if config.TickInterval == 0 {
		panic("tick interval must be greater than zero")
	}

	return &SmartBarrier{
		capacity:     capacity,
		capacityInfo: fmt.Sprintf("auto (initial=%d, min=%d, max=%d)", capacity, config.MinConcurrency, config.MaxConcurrency),
		sem:          sem,
		config:       &config,
		tickInterval: time.Duration(config.TickInterval) * time.Second,
		log:          l,
	}
}

func (b *SmartBarrier) Acquire() {
	b.callsCount.Add(1)
	b.pendingNum.Add(1)
	<-b.sem
	b.pendingNum.Add(-1)
}

func (b *SmartBarrier) Release() {
	b.sem <- struct{}{}
}

func (b *SmartBarrier) BusyCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// The number of in-flight request is the
	// the number of current capacity
	// minus the size of the semaphore channel
	return b.capacity - len(b.sem)
}

func (b *SmartBarrier) Capacity() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.capacity
}

func (b *SmartBarrier) CapacityInfo() string {
	return b.capacityInfo
}

func (b *SmartBarrier) HasDynamicCapacity() bool { return true }

func (b *SmartBarrier) Exhausted() {
	b.exhaustedCount.Add(1)
}

func (b *SmartBarrier) Start() {
	b.tick()
}

func (b *SmartBarrier) Stop() {
	b.close()
}

func (b *SmartBarrier) tick() {
	b.mu.RLock()
	cap := int64(b.capacity)
	state := b.state

	if b.tickTimer != nil {
		b.tickTimer.Stop()
	}

	b.mu.RUnlock()

	pending := b.pendingNum.Load()
	total := b.callsCount.Swap(0)
	exhausted := b.exhaustedCount.Swap(0)

	grow := false
	shrink := false

	if (float64(exhausted) / float64(total)) > b.config.ExhaustedFactor {
		shrink = true
	}

	if !shrink && (float64(pending+cap)/float64(cap)) > b.config.PendingFactor {
		grow = true
	}

	defer func() {
		b.tickTimer = time.AfterFunc(b.tickInterval, b.tick)
	}()

	// idle
	if state == 0 {
		if grow && !shrink {
			b.grow()
		}

		if !grow && shrink {
			b.shrink()
		}
		return
	}

	// growing
	if state == 1 {
		if grow && !shrink {
			b.grow()
			return
		}

		b.idle()
		return
	}

	// shrinking
	if state == -1 {
		if !grow && shrink {
			b.shrink()
			return
		}

		b.idle()
		return
	}
}

func (b *SmartBarrier) grow() {
	b.mu.Lock()

	cap := int(math.Round(float64(b.capacity) * b.config.GrowthFactor))

	if cap > b.config.MaxConcurrency {
		b.log.Warn("cannot increase concurrency, max reached", "capacity", b.capacity)
		b.mu.Unlock()
		return
	}

	b.log.Info("concurrency adjusted", "old", b.capacity, "new", cap)

	delta := cap - b.capacity

	b.capacity = cap
	b.state = 1
	b.mu.Unlock()

	for delta > 0 {
		b.sem <- struct{}{}
		delta--
	}
}

func (b *SmartBarrier) shrink() {
	b.mu.Lock()

	cap := int(math.Round(float64(b.capacity) / b.config.GrowthFactor))

	if cap < b.config.MinConcurrency {
		b.log.Warn("cannot decrease concurrency, min reached", "capacity", b.capacity)
		b.mu.Unlock()
		return
	}

	b.log.Info("concurrency adjusted", "old", b.capacity, "new", cap)

	delta := b.capacity - cap

	b.capacity = cap
	b.state = -1

	b.mu.Unlock()

	for delta > 0 {
		<-b.sem
		delta--
	}
}

func (b *SmartBarrier) idle() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state = 0
}

func (b *SmartBarrier) close() {
	if b.tickTimer != nil {
		b.tickTimer.Stop()
	}

	busy := b.BusyCount()

	for i := 0; i < busy; i++ {
		b.Release()
	}
}
