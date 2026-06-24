package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/anycable/anycable-go/utils"
	"github.com/jackc/pgx/v5"
)

// Listener owns a dedicated Postgres LISTEN connection for one wake-up channel.
type Listener struct {
	config  *Config
	channel string
	log     *slog.Logger
	wake    func(string)

	ctx    context.Context
	cancel context.CancelFunc
	conn   *pgx.Conn
	done   chan struct{}

	mu         sync.Mutex
	runStarted bool
	closed     bool
}

// NewListener opens a dedicated connection and subscribes to the configured
// NOTIFY channel. Notifications are intentionally used only as wake-up signals;
// callers must fetch payloads from their tables.
func NewListener(parent context.Context, config *Config, channel string, log *slog.Logger, wake func(string)) (*Listener, error) {
	ctx, cancel := context.WithCancel(parent)
	listener := &Listener{
		config:  config,
		channel: channel,
		log:     log,
		wake:    wake,
		ctx:     ctx,
		cancel:  cancel,
		done:    make(chan struct{}),
	}

	if err := listener.connect(); err != nil {
		cancel()
		return nil, err
	}

	return listener, nil
}

// Run waits for notifications and calls wake for every received signal. If the
// connection drops, it reconnects and re-issues LISTEN.
func (l *Listener) Run(done chan error) {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	l.runStarted = true
	l.mu.Unlock()

	defer close(l.done)
	defer l.closeConn(context.Background()) // nolint:errcheck

	attempt := 0

	for {
		notification, err := l.conn.WaitForNotification(l.ctx)
		if err == nil {
			attempt = 0
			l.log.With("channel", notification.Channel).Debug("received Postgres notification")
			l.wake(notification.Payload)
			continue
		}

		if errors.Is(err, context.Canceled) || l.ctx.Err() != nil {
			return
		}

		l.log.Warn("Postgres listener disconnected", "error", err)

		if err := l.reconnect(&attempt); err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}

			done <- err
			return
		}
	}
}

// Shutdown stops the listener and closes the dedicated connection.
func (l *Listener) Shutdown(ctx context.Context) error {
	l.cancel()

	l.mu.Lock()
	if !l.runStarted {
		l.closed = true
		l.mu.Unlock()
		return l.closeConn(ctx)
	}
	done := l.done
	l.mu.Unlock()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *Listener) connect() error {
	conn, err := pgx.Connect(l.ctx, l.config.URL)
	if err != nil {
		return fmt.Errorf("failed to connect Postgres listener: %w", err)
	}

	channel, err := QuoteIdentifier(l.channel)
	if err != nil {
		_ = conn.Close(context.Background())
		return err
	}

	if _, err := conn.Exec(l.ctx, fmt.Sprintf("LISTEN %s", channel)); err != nil {
		_ = conn.Close(context.Background())
		return fmt.Errorf("failed to listen on Postgres channel %s: %w", l.channel, err)
	}

	l.conn = conn
	l.log.With("channel", l.channel).Info("listening for Postgres notifications")

	return nil
}

func (l *Listener) reconnect(attempt *int) error {
	if l.conn != nil {
		_ = l.conn.Close(context.Background())
		l.conn = nil
	}

	for {
		(*attempt)++
		delay := utils.NextRetry(*attempt - 1)
		l.log.Info(fmt.Sprintf("next Postgres listener reconnect attempt in %s", delay))

		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-l.ctx.Done():
			timer.Stop()
			return l.ctx.Err()
		}

		if err := l.connect(); err != nil {
			l.log.Error("failed to reconnect Postgres listener", "error", err)
			continue
		}

		l.log.Info("reconnected Postgres listener")
		return nil
	}
}

func (l *Listener) closeConn(ctx context.Context) error {
	if l.conn == nil {
		return nil
	}

	err := l.conn.Close(ctx)
	l.conn = nil
	return err
}
