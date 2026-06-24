package pubsub

import (
	"context"
	"encoding/json"
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

type postgresNotification struct {
	V      int    `json:"v"`
	Stream string `json:"stream"`
	Offset int64  `json:"offset"`
}

// PostgresSubscriber distributes node-to-node publications through Postgres
// rows while using NOTIFY only as a polling wake-up signal.
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
	changed       map[string]struct{}
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
		changed:        make(map[string]struct{}),
		log:            l.With("context", "pubsub").With("provider", "postgres"),
		wakeCh:         make(chan struct{}, 1),
		trackingEvents: false,
		events:         make(map[string]subscriptionCmd),
	}, nil
}

// Start ensures the shared schema, subscribes to wake-up notifications, and
// starts the cursor polling loop. NOTIFY only scopes catch-up work; rows and
// local cursors remain the source of truth.
func (s *PostgresSubscriber) Start(done chan error) error {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	var pool *pgxpool.Pool
	var listener *pgadapter.Listener

	err := pgadapter.StartupWithRetry(s.ctx, s.config, s.log, "pub/sub adapter", func(ctx context.Context) error {
		nextPool, err := pgadapter.NewPool(ctx, s.config)
		if err != nil {
			return err
		}

		if err := pgadapter.EnsureSchema(ctx, nextPool, s.config); err != nil {
			nextPool.Close()
			return err
		}

		nextListener, err := pgadapter.NewListener(ctx, s.config, s.config.PubSubNotifyChannel, s.log, s.wakePayload)
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

// Subscribe registers local interest in a stream starting from the current
// stream tail.
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

// Unsubscribe removes local interest in a stream.
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

// Broadcast publishes a stream message through the Postgres pub/sub log.
func (s *PostgresSubscriber) Broadcast(msg *common.StreamMessage) {
	s.publish(msg.Stream, msg)
}

// BroadcastCommand publishes a remote command through the configured internal
// stream.
func (s *PostgresSubscriber) BroadcastCommand(cmd *common.RemoteCommandMessage) {
	s.publish(s.config.InternalStream, cmd)
}

// publish allocates the next per-stream pub/sub offset and inserts the fanout
// row in one statement so cleanup never resets stream ordering metadata.
func (s *PostgresSubscriber) publish(stream string, msg interface{}) {
	pool := s.currentPool()
	if pool == nil {
		return
	}

	query, err := newPostgresPubSubPublishSQL(s.config)
	if err != nil {
		s.log.Error("invalid Postgres pub/sub SQL identifiers", "error", err)
		return
	}

	payload := string(utils.ToJSON(msg))

	s.log.With("stream", stream).Debug("publish Postgres pub/sub message", "data", msg)

	if _, err := pool.Exec(context.Background(), query, stream, payload); err != nil {
		s.log.Error("failed to publish Postgres pub/sub message", "stream", stream, "error", err)
		return
	}

	s.wakeStream(stream)
}

func (s *PostgresSubscriber) pollLoop() {
	ticker := time.NewTicker(s.config.PollInterval())
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.wakeCh:
			s.pollOnce(false)
		case <-ticker.C:
			s.pollOnce(true)
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

func (s *PostgresSubscriber) pollOnce(all bool) {
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

	if _, err := s.pollStreams(pool, s.subscriptionSnapshot(all)); err != nil && s.ctx.Err() == nil {
		s.log.Error("failed to poll Postgres pub/sub streams", "error", err)
	}
}

// pollStreams batches local stream cursors and interleaves results by per-stream
// position so one busy stream cannot consume the whole batch.
func (s *PostgresSubscriber) pollStreams(pool *pgxpool.Pool, cursors map[string]int64) (int, error) {
	if len(cursors) == 0 {
		return 0, nil
	}

	queries, err := newPostgresPubSubQueries(s.config)
	if err != nil {
		return 0, err
	}

	streams := make([]string, 0, len(cursors))
	offsets := make([]int64, 0, len(cursors))
	for stream, cursor := range cursors {
		streams = append(streams, stream)
		offsets = append(offsets, cursor)
	}

	rows, err := pool.Query(s.ctx, queries.poll, streams, offsets, s.config.BatchLimit())
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var stream string
		var offset int64
		var payload string

		if err := rows.Scan(&stream, &offset, &payload); err != nil {
			return count, err
		}

		count++

		if !s.isSubscribed(stream) {
			// A snapshot can race an unsubscribe. Once local interest is gone,
			// there is no cursor left to advance for that stream.
			continue
		}

		// deliver logs malformed payloads as a terminal local outcome. Advancing
		// after the call keeps a poison row from pinning this node's cursor.
		s.deliver(stream, []byte(payload))
		s.advanceCursor(stream, offset)
	}

	return count, rows.Err()
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
	case common.RemoteCommandMessage:
		s.log.With("stream", stream).Debug("received Postgres pub/sub command")
		s.node.ExecuteRemoteCommand(&v)
	}
}

// cleanup trims old fanout rows only. Stream offsets remain in the shared
// offsets table so new publications continue from the last allocated cursor.
func (s *PostgresSubscriber) cleanup() error {
	pool := s.currentPool()
	if pool == nil {
		return nil
	}

	queries, err := newPostgresPubSubQueries(s.config)
	if err != nil {
		return err
	}

	_, err = pool.Exec(
		s.ctx,
		queries.cleanup,
		int64(s.config.RetentionDuration().Seconds()),
	)
	return err
}

// currentCursor starts new local subscriptions at the current stream tail. The
// Postgres pub/sub table is a catch-up log, not a durable replay source.
func (s *PostgresSubscriber) currentCursor(stream string) int64 {
	pool := s.currentPool()
	if pool == nil {
		return 0
	}

	queries, err := newPostgresPubSubQueries(s.config)
	if err != nil {
		s.log.Error("invalid Postgres pub/sub SQL identifiers", "error", err)
		return 0
	}

	var cursor int64
	if err := pool.QueryRow(context.Background(), queries.currentCursor, stream).Scan(&cursor); err != nil {
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

// subscriptionSnapshot returns either all subscribed cursors for fallback polls
// or the coalesced streams marked by NOTIFY wakeups since the last snapshot.
func (s *PostgresSubscriber) subscriptionSnapshot(all bool) map[string]int64 {
	s.subMu.Lock()
	defer s.subMu.Unlock()

	if all || len(s.changed) == 0 {
		snapshot := make(map[string]int64, len(s.subscriptions))
		for stream, entry := range s.subscriptions {
			snapshot[stream] = entry.cursor
		}
		s.changed = make(map[string]struct{})
		return snapshot
	}

	snapshot := make(map[string]int64, len(s.changed))
	for stream := range s.changed {
		if entry, ok := s.subscriptions[stream]; ok {
			snapshot[stream] = entry.cursor
		}
	}
	s.changed = make(map[string]struct{})

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

func (s *PostgresSubscriber) isSubscribed(stream string) bool {
	s.subMu.RLock()
	defer s.subMu.RUnlock()

	_, ok := s.subscriptions[stream]
	return ok
}

func (s *PostgresSubscriber) wake() {
	select {
	case s.wakeCh <- struct{}{}:
	default:
	}
}

func (s *PostgresSubscriber) wakePayload(payload string) {
	var notification postgresNotification
	if err := json.Unmarshal([]byte(payload), &notification); err != nil || notification.V != 1 || notification.Stream == "" {
		s.log.Warn("failed to parse Postgres pub/sub notification", "payload", logger.CompactValue(payload), "error", err)
		s.wake()
		return
	}

	// The notification offset is a latency hint only. Cursor-based polling owns
	// correctness, so we only use the stream name to scope catch-up work.
	s.wakeStream(notification.Stream)
}

// wakeStream records a coalesced catch-up request for subscribed streams. The
// polling loop drains the changed set under subscriptionSnapshot.
func (s *PostgresSubscriber) wakeStream(stream string) {
	s.subMu.Lock()
	if _, ok := s.subscriptions[stream]; !ok {
		s.subMu.Unlock()
		return
	}
	s.changed[stream] = struct{}{}
	s.subMu.Unlock()

	s.wake()
}

// postgresPubSubQueries contains SQL rendered with a validated, quoted pub/sub
// table identifier. Runtime values continue to flow through query parameters.
type postgresPubSubQueries struct {
	poll          string
	cleanup       string
	currentCursor string
}

func newPostgresPubSubQueries(config *pgadapter.Config) (postgresPubSubQueries, error) {
	pubsubTable, err := pgadapter.QuoteTableName(config.PubSubTable)
	if err != nil {
		return postgresPubSubQueries{}, fmt.Errorf("invalid Postgres pub/sub table: %w", err)
	}

	return postgresPubSubQueries{
		poll:          fmt.Sprintf(pollPostgresPubSubSQL, pubsubTable),
		cleanup:       fmt.Sprintf(cleanupPostgresPubSubSQL, pubsubTable),
		currentCursor: fmt.Sprintf(currentPostgresPubSubCursorSQL, pubsubTable),
	}, nil
}

func newPostgresPubSubPublishSQL(config *pgadapter.Config) (string, error) {
	pubsubTable, err := pgadapter.QuoteTableName(config.PubSubTable)
	if err != nil {
		return "", fmt.Errorf("invalid Postgres pub/sub table: %w", err)
	}

	offsetsTable, err := pgadapter.QuoteTableName(config.StreamOffsetsTable)
	if err != nil {
		return "", fmt.Errorf("invalid Postgres stream offsets table: %w", err)
	}

	return fmt.Sprintf(publishPostgresPubSubSQL, offsetsTable, pubsubTable), nil
}

const publishPostgresPubSubSQL = `
WITH next_offset AS (
  INSERT INTO %[1]s AS offsets (scope, stream, "offset")
  VALUES ('pubsub', $1, 1)
  ON CONFLICT (scope, stream)
  DO UPDATE SET "offset" = offsets."offset" + 1,
                updated_at = now()
  RETURNING "offset"
)
INSERT INTO %[2]s (stream, "offset", payload, meta)
SELECT $1, "offset", $2, '{}'
FROM next_offset
`

const pollPostgresPubSubSQL = `
WITH cursors AS (
  SELECT c.stream, c.cursor_offset
  FROM unnest($1::text[], $2::bigint[]) AS c(stream, cursor_offset)
),
publications AS (
  SELECT
    cursors.stream,
    rows."offset",
    rows.payload,
    row_number() OVER (PARTITION BY cursors.stream ORDER BY rows."offset") AS stream_position
  FROM cursors
  JOIN LATERAL (
    SELECT "offset", payload
    FROM %[1]s
    WHERE stream = cursors.stream
      AND "offset" > cursors.cursor_offset
    ORDER BY "offset"
    LIMIT $3
  ) rows ON true
)
SELECT stream, "offset", payload
FROM publications
ORDER BY stream_position, stream, "offset"
LIMIT $3
`

const cleanupPostgresPubSubSQL = `
DELETE FROM %[1]s
WHERE created_at < now() - ($1::bigint * interval '1 second')
`

const currentPostgresPubSubCursorSQL = `
SELECT COALESCE(MAX("offset"), 0)
FROM %[1]s
WHERE stream = $1
`

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
