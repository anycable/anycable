package utils

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type signalHandler func(ctx context.Context) error

// The struct implementing logic for graceful shutdown in response to OS signals.
type GracefulSignals struct {
	handlers              []signalHandler
	forceTerminateHandler func()
	timeout               time.Duration
	executed              bool

	ch chan os.Signal
	mu sync.Mutex
}

// Create new GracefulSignals struct.
func NewGracefulSignals(timeout time.Duration) *GracefulSignals {
	return &GracefulSignals{
		timeout:               timeout,
		forceTerminateHandler: func() { os.Exit(0) },
		handlers:              make([]signalHandler, 0),
		ch:                    make(chan os.Signal, 1),
	}
}

func (s *GracefulSignals) Handle(handler signalHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.handlers = append(s.handlers, handler)
}

func (s *GracefulSignals) HandleForceTerminate(handler func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.forceTerminateHandler = handler
}

func (s *GracefulSignals) Listen() {
	signal.Notify(s.ch, syscall.SIGINT, syscall.SIGTERM)
	go s.listen()
}

func (s *GracefulSignals) listen() {
	for range s.ch {
		s.exec()
	}
}

func (s *GracefulSignals) exec() {
	s.mu.Lock()

	if s.executed {
		s.mu.Unlock()
		return
	}

	shutdown := make(chan struct{})
	s.executed = true

	terminateCtx, terminateImmediately := context.WithCancel(context.Background())

	timeoutCtx, cancelTimeout := context.WithTimeout(terminateCtx, s.timeout)
	defer cancelTimeout()

	go func() {
		termSig := make(chan os.Signal, 1)
		signal.Notify(termSig, syscall.SIGINT, syscall.SIGTERM)
		<-termSig

		terminateImmediately()

		// Wait for handlers to interrupt
		// NOTE: it's the responsibility of handlers to react on the context cancellation
		<-shutdown

		s.mu.Lock()
		defer s.mu.Unlock()

		if s.forceTerminateHandler != nil {
			s.forceTerminateHandler()
		}
	}()

	handlers := s.handlers[:] // nolint:gocritic
	s.mu.Unlock()

	for _, handler := range handlers {
		handler(timeoutCtx) // nolint:errcheck
	}

	close(shutdown)
}
