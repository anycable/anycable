package utils

import (
	"fmt"
	"time"
)

// ErrScheduleTimeout returned by Pool to indicate that there no free
// goroutines during some period of time.
var ErrScheduleTimeout = fmt.Errorf("schedule error: timed out")

// GoPool contains logic of goroutine reuse.
// Copied from https://github.com/gobwas/ws-examples/blob/master/src/gopool/pool.go
type GoPool struct {
	sem  chan struct{}
	work chan func()
}

// NewGoPool creates new goroutine pool with given size.
// Start size defaults to 20% of the max size.
// Queue size defaults to 10% of the max size.
func NewGoPool(size int) *GoPool {
	spawn := int(size / 5)
	queue := int(size / 10)

	if spawn <= 0 {
		spawn = 1
	}

	if queue <= 0 {
		queue = 1
	}

	p := &GoPool{
		sem:  make(chan struct{}, size),
		work: make(chan func(), queue),
	}

	for i := 0; i < spawn; i++ {
		p.sem <- struct{}{}
		go p.worker(func() {})
	}

	return p
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
	defer func() { <-p.sem }()

	task()

	for task := range p.work {
		task()
	}
}
