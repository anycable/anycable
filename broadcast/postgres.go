package broadcast

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	pgadapter "github.com/anycable/anycable-go/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresBroadcaster claims app-published broadcast rows from Postgres and
// passes each row to exactly one AnyCable node.
type PostgresBroadcaster struct {
	node   Handler
	config *pgadapter.Config

	pool     *pgxpool.Pool
	listener *pgadapter.Listener

	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once

	wakeCh chan struct{}

	log *slog.Logger
}

var _ Broadcaster = (*PostgresBroadcaster)(nil)

// NewPostgresBroadcaster consumes broadcast payloads from Postgres. It is a
// single-consumer adapter, so multi-node deployments should pair it with
// pubsub=postgres.
func NewPostgresBroadcaster(node Handler, config *pgadapter.Config, l *slog.Logger) *PostgresBroadcaster {
	return &PostgresBroadcaster{
		node:   node,
		config: config,
		log:    l.With("context", "broadcast").With("provider", "postgres"),
		wakeCh: make(chan struct{}, 1),
	}
}

// IsFanout is false because a broadcast row must be claimed by exactly one
// AnyCable node before the node re-distributes it through the broker/pubsub path.
func (*PostgresBroadcaster) IsFanout() bool {
	return false
}

// Start ensures the schema, subscribes to wake-up
// notifications, and starts the polling/cleanup loops.
func (b *PostgresBroadcaster) Start(done chan error) error {
	if err := pgadapter.ValidateIdentifiers(b.config); err != nil {
		return err
	}

	b.ctx, b.cancel = context.WithCancel(context.Background())

	var pool *pgxpool.Pool
	var listener *pgadapter.Listener

	err := pgadapter.StartupWithRetry(b.ctx, b.config, b.log, "broadcast adapter", func(ctx context.Context) error {
		nextPool, err := pgadapter.NewPool(ctx, b.config)
		if err != nil {
			return err
		}

		if err := pgadapter.EnsureSchema(ctx, nextPool, b.config); err != nil {
			nextPool.Close()
			return err
		}

		nextListener, err := pgadapter.NewListener(ctx, b.config, b.config.BroadcastNotifyChannel, b.log, func(string) {
			b.wake()
		})
		if err != nil {
			nextPool.Close()
			return err
		}

		pool = nextPool
		listener = nextListener
		return nil
	})
	if err != nil {
		b.cancel()
		return err
	}

	b.pool = pool
	b.listener = listener

	b.log.Info("starting Postgres broadcast adapter", "table", b.config.BroadcastsTable)

	b.wake()
	go listener.Run(done)
	go b.pollLoop()
	go b.cleanupLoop()

	return nil
}

// Shutdown stops the loops and closes the listener and pool.
func (b *PostgresBroadcaster) Shutdown(ctx context.Context) error {
	b.once.Do(func() {
		if b.cancel != nil {
			b.cancel()
		}
	})

	if b.listener != nil {
		if err := b.listener.Shutdown(ctx); err != nil {
			return err
		}
	}

	if b.pool != nil {
		b.pool.Close()
	}

	return nil
}

func (b *PostgresBroadcaster) pollLoop() {
	ticker := time.NewTicker(b.config.PollInterval())
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-b.wakeCh:
			b.drain()
		case <-ticker.C:
			b.drain()
		}
	}
}

func (b *PostgresBroadcaster) cleanupLoop() {
	interval := b.config.CleanupDuration()
	if interval <= 0 {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			if err := b.cleanup(); err != nil {
				b.log.Warn("failed to cleanup old Postgres broadcast rows", "error", err)
			}
		}
	}
}

func (b *PostgresBroadcaster) drain() {
	for {
		count, err := b.pollOnce()
		if err != nil {
			if b.ctx.Err() != nil {
				return
			}
			b.log.Error("failed to poll Postgres broadcasts", "error", err)
			return
		}

		if count == 0 || count < b.config.BatchLimit() {
			return
		}
	}
}

func (b *PostgresBroadcaster) pollOnce() (int, error) {
	table, err := pgadapter.QuoteTableName(b.config.BroadcastsTable)
	if err != nil {
		return 0, err
	}

	if err := b.finalizeExpiredFinalAttempts(table); err != nil {
		return 0, err
	}

	query := fmt.Sprintf(`
WITH candidates AS (
  SELECT broadcasts.stream, broadcasts."offset"
  FROM %s broadcasts
  WHERE broadcasts.attempts < $4
    AND broadcasts.exhausted_at IS NULL
    AND (
      broadcasts.claimed_at IS NULL
      OR broadcasts.claimed_at < now() - ($1::bigint * interval '1 second')
    )
    AND NOT EXISTS (
      SELECT 1
      FROM %s older
      WHERE older.stream = broadcasts.stream
        AND older."offset" < broadcasts."offset"
        AND (
          older.exhausted_at IS NULL
          OR $5 = 'block'
        )
    )
  ORDER BY broadcasts.created_at, broadcasts.stream, broadcasts."offset"
  LIMIT $2
  FOR UPDATE SKIP LOCKED
)
UPDATE %s AS broadcasts
SET claimed_by = $3,
    claimed_at = now(),
    attempts = broadcasts.attempts + 1,
    last_error = NULL
FROM candidates
WHERE broadcasts.stream = candidates.stream
  AND broadcasts."offset" = candidates."offset"
RETURNING broadcasts.stream, broadcasts."offset", broadcasts.payload, broadcasts.attempts
`, table, table, table)

	rows, err := b.pool.Query(b.ctx, query, b.config.ClaimTimeout(), b.config.BatchLimit(), b.config.NodeID(), b.config.AttemptsLimit(), b.config.ExhaustedPolicy())
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var stream string
		var offset int64
		var payload string
		var attempts int

		if err := rows.Scan(&stream, &offset, &payload, &attempts); err != nil {
			return count, err
		}

		count++

		if err := b.node.HandleBroadcast([]byte(payload)); err != nil {
			b.log.Warn("failed to handle Postgres broadcast", "stream", stream, "offset", offset, "attempts", attempts, "error", err)

			if attempts >= b.config.AttemptsLimit() {
				if err := b.fail(stream, offset, err); err != nil {
					return count, err
				}
			} else {
				if err := b.release(stream, offset, err); err != nil {
					return count, err
				}
			}

			continue
		}

		if err := b.ack(stream, offset); err != nil {
			return count, err
		}
	}

	if err := rows.Err(); err != nil {
		return count, err
	}

	return count, nil
}

func (b *PostgresBroadcaster) finalizeExpiredFinalAttempts(table string) error {
	query := fmt.Sprintf(`
UPDATE %s
SET exhausted_at = now(),
    last_error = COALESCE(last_error, 'claim timed out on final attempt')
WHERE exhausted_at IS NULL
  AND attempts >= $1
  AND claimed_at IS NOT NULL
  AND claimed_at < now() - ($2::bigint * interval '1 second')
`, table)

	_, err := b.pool.Exec(b.ctx, query, b.config.AttemptsLimit(), b.config.ClaimTimeout())
	return err
}

func (b *PostgresBroadcaster) ack(stream string, offset int64) error {
	table, err := pgadapter.QuoteTableName(b.config.BroadcastsTable)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE stream = $1 AND \"offset\" = $2 AND claimed_by = $3", table)
	_, err = b.pool.Exec(b.ctx, query, stream, offset, b.config.NodeID())
	return err
}

func (b *PostgresBroadcaster) release(stream string, offset int64, cause error) error {
	return b.updateFailure(stream, offset, cause, false)
}

func (b *PostgresBroadcaster) fail(stream string, offset int64, cause error) error {
	return b.updateFailure(stream, offset, cause, true)
}

func (b *PostgresBroadcaster) updateFailure(stream string, offset int64, cause error, final bool) error {
	table, err := pgadapter.QuoteTableName(b.config.BroadcastsTable)
	if err != nil {
		return err
	}

	lastError := cause.Error()
	if len(lastError) > 2048 {
		lastError = lastError[:2048]
	}

	if final {
		b.log.Error("Postgres broadcast attempts exhausted", "stream", stream, "offset", offset, "error", lastError)
	}

	// Non-final failures clear the claim so the row can be retried later. Final
	// failures are left in the table until cleanup for operator inspection.
	if final {
		query := fmt.Sprintf(`
UPDATE %s
SET last_error = $3,
    exhausted_at = now()
WHERE stream = $1
  AND "offset" = $2
  AND claimed_by = $4
`, table)

		_, err = b.pool.Exec(b.ctx, query, stream, offset, lastError, b.config.NodeID())
		return err
	}

	query := fmt.Sprintf(`
UPDATE %s
SET claimed_by = NULL,
    claimed_at = NULL,
    last_error = $3
WHERE stream = $1
  AND "offset" = $2
  AND claimed_by = $4
`, table)

	_, err = b.pool.Exec(b.ctx, query, stream, offset, lastError, b.config.NodeID())
	return err
}

func (b *PostgresBroadcaster) cleanup() error {
	table, err := pgadapter.QuoteTableName(b.config.BroadcastsTable)
	if err != nil {
		return err
	}

	if b.config.ExhaustedPolicy() == pgadapter.ExhaustedBroadcastPolicyBlock {
		return nil
	}

	query := fmt.Sprintf(`
DELETE FROM %s
WHERE exhausted_at IS NOT NULL
  AND exhausted_at < now() - ($1::bigint * interval '1 second')
`, table)

	_, err = b.pool.Exec(b.ctx, query, int64(b.config.RetentionDuration().Seconds()))
	return err
}

func (b *PostgresBroadcaster) wake() {
	select {
	case b.wakeCh <- struct{}{}:
	default:
	}
}

// String returns a human-readable adapter identifier.
func (b *PostgresBroadcaster) String() string {
	return strings.Join([]string{"postgres", b.config.BroadcastsTable}, ":")
}
