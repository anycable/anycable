package pubsub

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/logger"
	pgadapter "github.com/anycable/anycable-go/postgres"
	"github.com/anycable/anycable-go/utils"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresSubscriptionEntry struct {
	cursor int64
}

type PostgresSubscriber struct {
	node   Handler
	config *pgadapter.Config

	pool   *pgxpool.Pool
	poolMu sync.RWMutex

	listener *pgadapter.Listener

	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once

	subscriptions map[string]*postgresSubscriptionEntry
	subMu         sync.RWMutex
	pollMu        sync.Mutex
	wakeCh        chan struct{}

	log *slog.Logger

	// Test-only subscription events mirror RedisSubscriber's shared test hook.
	events         map[string]subscriptionCmd
	eventsMu       sync.Mutex
	trackingEvents bool
}

var _ Subscriber = (*PostgresSubscriber)(nil)

// NewPostgresSubscriber creates a database-backed multi-node subscriber. Rows
// carry the full payload; NOTIFY only wakes the polling loop.
func NewPostgresSubscriber(node Handler, config *pgadapter.Config, l *slog.Logger) (*PostgresSubscriber, error) {
	if err := pgadapter.ValidateIdentifiers(config); err != nil {
		return nil, err
	}

	return &PostgresSubscriber{
		node:           node,
		config:         config,
		subscriptions:  make(map[string]*postgresSubscriptionEntry),
		log:            l.With("context", "pubsub").With("provider", "postgres"),
		wakeCh:         make(chan struct{}, 1),
		trackingEvents: false,
		events:         make(map[string]subscriptionCmd),
	}, nil
}

// Start validates the shared contract and starts the cursor polling loop.
func (s *PostgresSubscriber) Start(done chan error) error {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	var pool *pgxpool.Pool
	var listener *pgadapter.Listener

	err := pgadapter.StartupWithRetry(s.ctx, s.config, s.log, "pub/sub adapter", func(ctx context.Context) error {
		nextPool, err := pgadapter.NewPool(ctx, s.config)
		if err != nil {
			return err
		}

		if err := pgadapter.ValidateContract(ctx, nextPool, s.config); err != nil {
			nextPool.Close()
			return err
		}

		nextListener, err := pgadapter.NewListener(ctx, s.config, s.log, s.wake)
		if err != nil {
			nextPool.Close()
			return err
		}

		pool = nextPool
		listener = nextListener
		return nil
	})
	if err != nil {
		s.cancel()
		return err
	}

	s.poolMu.Lock()
	s.pool = pool
	s.poolMu.Unlock()

	s.Subscribe(s.config.InternalStream)

	s.listener = listener

	s.log.Info("starting Postgres pub/sub adapter", "table", s.config.PubSubTable)

	s.wake()
	go listener.Run(done)
	go s.pollLoop()
	go s.cleanupLoop()

	return nil
}

// Shutdown stops polling and closes the listener and pool.
func (s *PostgresSubscriber) Shutdown(ctx context.Context) error {
	s.once.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
	})

	if s.listener != nil {
		if err := s.listener.Shutdown(ctx); err != nil {
			return err
		}
	}

	s.poolMu.Lock()
	if s.pool != nil {
		s.pool.Close()
		s.pool = nil
	}
	s.poolMu.Unlock()

	return nil
}

// IsMultiNode returns true because publications are written to shared storage
// and can be consumed by all interested nodes.
func (*PostgresSubscriber) IsMultiNode() bool {
	return true
}

func (s *PostgresSubscriber) Subscribe(stream string) {
	// Subscriptions start at the current tail. The pub/sub table is not a
	// durable replay log for newly subscribed streams.
	cursor := s.currentCursor(stream)

	s.subMu.Lock()
	s.subscriptions[stream] = &postgresSubscriptionEntry{cursor: cursor}
	s.subMu.Unlock()

	s.log.With("stream", stream).Debug("subscribed to Postgres pub/sub stream", "cursor", cursor)
	s.trackEvent("subscribe", stream)
}

func (s *PostgresSubscriber) Unsubscribe(stream string) {
	s.subMu.Lock()
	if _, ok := s.subscriptions[stream]; !ok {
		s.subMu.Unlock()
		return
	}

	delete(s.subscriptions, stream)
	s.subMu.Unlock()

	s.log.With("stream", stream).Debug("unsubscribed from Postgres pub/sub stream")
	s.trackEvent("unsubscribe", stream)
}

func (s *PostgresSubscriber) Broadcast(msg *common.StreamMessage) {
	s.publish(msg.Stream, msg)
}

func (s *PostgresSubscriber) BroadcastCommand(cmd *common.RemoteCommandMessage) {
	s.publish(s.config.InternalStream, cmd)
}

func (s *PostgresSubscriber) publish(stream string, msg interface{}) {
	pool := s.currentPool()
	if pool == nil {
		return
	}

	table, err := pgadapter.QuoteTableName(s.config.PubSubTable)
	if err != nil {
		s.log.Error("invalid Postgres pub/sub table", "error", err)
		return
	}

	payload := string(utils.ToJSON(msg))
	query := fmt.Sprintf("INSERT INTO %s (stream, payload) VALUES ($1, $2)", table)

	s.log.With("stream", stream).Debug("publish Postgres pub/sub message", "data", msg)

	if _, err := pool.Exec(context.Background(), query, stream, payload); err != nil {
		s.log.Error("failed to publish Postgres pub/sub message", "stream", stream, "error", err)
		return
	}

	s.wake()
}

func (s *PostgresSubscriber) pollLoop() {
	ticker := time.NewTicker(s.config.PollInterval())
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.wakeCh:
			s.pollOnce()
		case <-ticker.C:
			s.pollOnce()
		}
	}
}

func (s *PostgresSubscriber) cleanupLoop() {
	interval := s.config.CleanupDuration()
	if interval <= 0 {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.cleanup(); err != nil {
				s.log.Warn("failed to cleanup old Postgres pub/sub rows", "error", err)
			}
		}
	}
}

func (s *PostgresSubscriber) pollOnce() {
	// Serialize polls so a slow database round-trip cannot race another wake-up
	// and deliver the same row twice to this node.
	if !s.pollMu.TryLock() {
		return
	}
	defer s.pollMu.Unlock()

	pool := s.currentPool()
	if pool == nil {
		return
	}

	snapshot := s.subscriptionSnapshot()
	for stream, cursor := range snapshot {
		if err := s.pollStream(pool, stream, cursor); err != nil {
			s.log.Error("failed to poll Postgres pub/sub stream", "stream", stream, "error", err)
		}
	}
}

func (s *PostgresSubscriber) pollStream(pool *pgxpool.Pool, stream string, cursor int64) error {
	table, err := pgadapter.QuoteTableName(s.config.PubSubTable)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`
SELECT id, payload
FROM %s
WHERE stream = $1
  AND id > $2
ORDER BY id
LIMIT $3
`, table)

	rows, err := pool.Query(s.ctx, query, stream, cursor, s.config.BatchLimit())
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var payload string

		if err := rows.Scan(&id, &payload); err != nil {
			return err
		}

		if !s.advanceCursor(stream, id) {
			return nil
		}

		s.deliver(stream, []byte(payload))
	}

	return rows.Err()
}

func (s *PostgresSubscriber) deliver(stream string, payload []byte) {
	msg, err := common.PubSubMessageFromJSON(payload)
	if err != nil {
		s.log.Warn("failed to parse Postgres pub/sub message", "stream", stream, "data", logger.CompactValue(string(payload)), "error", err)
		return
	}

	switch v := msg.(type) {
	case common.StreamMessage:
		s.log.With("stream", stream).Debug("received Postgres pub/sub broadcast")
		s.node.Broadcast(&v)
	case []*common.StreamMessage:
		for _, msg := range v {
			s.log.With("stream", stream).Debug("received Postgres pub/sub broadcast")
			s.node.Broadcast(msg)
		}
	case common.RemoteCommandMessage:
		s.log.With("stream", stream).Debug("received Postgres pub/sub command")
		s.node.ExecuteRemoteCommand(&v)
	}
}

func (s *PostgresSubscriber) cleanup() error {
	pool := s.currentPool()
	if pool == nil {
		return nil
	}

	table, err := pgadapter.QuoteTableName(s.config.PubSubTable)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`
DELETE FROM %s
WHERE created_at < now() - ($1::bigint * interval '1 second')
`, table)

	_, err = pool.Exec(s.ctx, query, int64(s.config.RetentionDuration().Seconds()))
	return err
}

func (s *PostgresSubscriber) currentCursor(stream string) int64 {
	pool := s.currentPool()
	if pool == nil {
		return 0
	}

	table, err := pgadapter.QuoteTableName(s.config.PubSubTable)
	if err != nil {
		s.log.Error("invalid Postgres pub/sub table", "error", err)
		return 0
	}

	query := fmt.Sprintf("SELECT COALESCE(MAX(id), 0) FROM %s WHERE stream = $1", table)

	var cursor int64
	if err := pool.QueryRow(context.Background(), query, stream).Scan(&cursor); err != nil {
		s.log.Warn("failed to initialize Postgres pub/sub cursor", "stream", stream, "error", err)
		return 0
	}

	return cursor
}

func (s *PostgresSubscriber) currentPool() *pgxpool.Pool {
	s.poolMu.RLock()
	defer s.poolMu.RUnlock()

	return s.pool
}

func (s *PostgresSubscriber) subscriptionSnapshot() map[string]int64 {
	s.subMu.RLock()
	defer s.subMu.RUnlock()

	snapshot := make(map[string]int64, len(s.subscriptions))
	for stream, entry := range s.subscriptions {
		snapshot[stream] = entry.cursor
	}

	return snapshot
}

func (s *PostgresSubscriber) advanceCursor(stream string, id int64) bool {
	s.subMu.Lock()
	defer s.subMu.Unlock()

	entry, ok := s.subscriptions[stream]
	if !ok {
		return false
	}

	if id > entry.cursor {
		entry.cursor = id
	}

	return true
}

func (s *PostgresSubscriber) wake() {
	select {
	case s.wakeCh <- struct{}{}:
	default:
	}
}

// test-only
func (s *PostgresSubscriber) trackEvent(event string, channel string) {
	if !s.trackingEvents {
		return
	}

	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()

	if event == "subscribe" {
		s.events[channel] = subscribeCmd
	} else if event == "unsubscribe" {
		s.events[channel] = unsubscribeCmd
	}
}

// test-only
func (s *PostgresSubscriber) getEvent(channel string) subscriptionCmd {
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()

	cmd, ok := s.events[channel]
	if !ok {
		return unsubscribeCmd
	}

	return cmd
}
