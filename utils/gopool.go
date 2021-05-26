package utils

import (
	"fmt"
	"time"
)

// ErrScheduleTimeout returned by Pool to indicate that there no free
// goroutines during some period of time.
var ErrScheduleTimeout = fmt.Errorf("schedule error: timed out")

// How many tasks should a worker perform before "re-starting" the goroutine
// See https://adtac.in/2021/04/23/note-on-worker-pools-in-go.html
const workerRespawnThreshold = 1 << 16

// GoPool contains logic of goroutine reuse.
// Copied from https://github.com/gobwas/ws-examples/blob/master/src/gopool/pool.go
type GoPool struct {
	name string
	size int
	sem  chan struct{}
	work chan func()
}

var initializedPools []*GoPool = make([]*GoPool, 0)

// Return all active pools
func AllPools() []*GoPool {
	return initializedPools
}

// NewGoPool creates new goroutine pool with given size.
// Start size defaults to 20% of the max size but not greater than 1024.
// Queue size defaults to 50% of the max size.
func NewGoPool(name string, size int) *GoPool {
	spawn := int(size / 5)
	queue := int(size / 2)

	if spawn <= 0 {
		spawn = 1
	}

	if spawn > 1024 {
		spawn = 1024
	}

	if queue <= 0 {
		queue = 1
	}

	p := &GoPool{
		name: name,
		size: size,
		sem:  make(chan struct{}, size),
		work: make(chan func(), queue),
	}

	for i := 0; i < spawn; i++ {
		p.sem <- struct{}{}
		go p.worker(func() {})
	}

	initializedPools = append(initializedPools, p)

	return p
}

func (p *GoPool) Name() string {
	return p.name
}

func (p *GoPool) Size() int {
	return p.size
}

// Schedule schedules task to be executed over pool's workers.
func (p *GoPool) Schedule(task func()) {
	p.schedule(task, nil) // nolint:errcheck
}

// ScheduleTimeout schedules task to be executed over pool's workers.
// It returns ErrScheduleTimeout when no free workers met during given timeout.
func (p *GoPool) ScheduleTimeout(timeout time.Duration, task func()) error {
	return p.schedule(task, time.After(timeout))
}

func (p *GoPool) schedule(task func(), timeout <-chan time.Time) error {
	select {
	case <-timeout:
		return ErrScheduleTimeout
	case p.work <- task:
		return nil
	case p.sem <- struct{}{}:
		go p.worker(task)
		return nil
	}
}

func (p *GoPool) worker(task func()) {
	counter := 1
	defer func() { <-p.sem }()

	task()

	for task := range p.work {
		task()
		counter++

		if counter >= workerRespawnThreshold {
			return
		}
	}
}
