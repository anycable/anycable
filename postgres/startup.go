package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/anycable/anycable-go/utils"
)

type retryDelayFunc func(int) time.Duration

// StartupWithRetry retries startup checks that depend on a reachable database.
// It keeps invalid config/schema as startup errors, but tolerates short database
// readiness races during deploys or local service boot.
func StartupWithRetry(ctx context.Context, config *Config, log *slog.Logger, component string, start func(context.Context) error) error {
	return startupWithRetryDelay(ctx, config, log, component, start, utils.NextRetry)
}

func startupWithRetryDelay(ctx context.Context, config *Config, log *slog.Logger, component string, start func(context.Context) error, delayFor retryDelayFunc) error {
	attempts := config.StartupAttempts()
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		if err := start(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if attempt >= attempts {
			break
		}

		delay := delayFor(attempt - 1)
		log.Warn(
			"Postgres startup check failed",
			"component", component,
			"attempt", attempt,
			"max_attempts", attempts,
			"next_attempt_in", delay,
			"error", lastErr,
		)

		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}

	return fmt.Errorf("postgres %s startup failed after %d attempt(s): %w", component, attempts, lastErr)
}
