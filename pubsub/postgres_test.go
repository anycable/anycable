package pubsub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	pgadapter "github.com/anycable/anycable-go/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresSubscriber(t *testing.T) {
	config, pool := setupPostgresPubSubTest(t)
	defer pool.Close()
	defer dropPostgresPubSubTestTables(t, pool, config)

	SharedSubscriberTests(t, func(handler *TestHandler) Subscriber {
		subscriber, err := NewPostgresSubscriber(handler, config, slog.Default())
		require.NoError(t, err)
		subscriber.trackingEvents = true
		return subscriber
	}, waitPostgresSubscription)
}

func TestPostgresSubscriberNotificationScopesCatchup(t *testing.T) {
	config := pgadapter.NewConfig()
	subscriber, err := NewPostgresSubscriber(NewTestHandler(), &config, slog.Default())
	require.NoError(t, err)

	subscriber.Subscribe("alpha")
	subscriber.Subscribe("beta")

	subscriber.wakePayload(`{"v":1,"stream":"alpha","offset":10}`)

	assert.Equal(t, map[string]int64{"alpha": 0}, subscriber.subscriptionSnapshot(false))

	subscriber.wakePayload(`{"v":1,"stream":"gamma","offset":11}`)

	subscriber.subMu.RLock()
	_, changed := subscriber.changed["gamma"]
	subscriber.subMu.RUnlock()
	assert.False(t, changed)

	subscriber.wakePayload(`not-json`)

	assert.Equal(t, map[string]int64{"alpha": 0, "beta": 0}, subscriber.subscriptionSnapshot(false))
}

func TestPostgresSubscriberPollStreamsRespectsBatchLimit(t *testing.T) {
	config, pool := setupPostgresPubSubTest(t)
	defer pool.Close()
	defer dropPostgresPubSubTestTables(t, pool, config)

	config.BatchSize = 3
	require.NoError(t, pgadapter.EnsureSchema(context.Background(), pool, config))

	handler := NewTestHandler()
	subscriber, err := NewPostgresSubscriber(handler, config, slog.Default())
	require.NoError(t, err)
	subscriber.pool = pool
	subscriber.ctx = context.Background()

	for i := 0; i < 5; i++ {
		stream := fmt.Sprintf("batch_%d", i)
		subscriber.subscriptions[stream] = &postgresSubscriptionEntry{cursor: 0}
		subscriber.publish(stream, &common.StreamMessage{Stream: stream, Data: fmt.Sprintf("message_%d", i)})
	}

	count, err := subscriber.pollStreams(pool, subscriber.subscriptionSnapshot(true))
	require.NoError(t, err)
	assert.Equal(t, 3, count)
	assert.Len(t, handler.messages, 3)
}

func TestPostgresSubscriberMalformedPayloadAdvancesCursor(t *testing.T) {
	config, pool := setupPostgresPubSubTest(t)
	defer pool.Close()
	defer dropPostgresPubSubTestTables(t, pool, config)

	require.NoError(t, pgadapter.EnsureSchema(context.Background(), pool, config))

	handler := NewTestHandler()
	subscriber, err := NewPostgresSubscriber(handler, config, slog.Default())
	require.NoError(t, err)
	subscriber.pool = pool
	subscriber.ctx = context.Background()
	subscriber.subscriptions["bad"] = &postgresSubscriptionEntry{cursor: 0}

	pubsubTable, err := pgadapter.QuoteTableName(config.PubSubTable)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), fmt.Sprintf("INSERT INTO %s (stream, \"offset\", payload, meta) VALUES ($1, $2, $3, '{}')", pubsubTable), "bad", 1, "not-json")
	require.NoError(t, err)

	count, err := subscriber.pollStreams(pool, map[string]int64{"bad": 0})
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, int64(1), subscriber.subscriptions["bad"].cursor)
	assert.Len(t, handler.messages, 0)
}

func waitPostgresSubscription(subscriber Subscriber, stream string) error {
	postgresSubscriber, ok := subscriber.(*PostgresSubscriber)
	if !ok {
		return errors.New("subscriber is not a PostgresSubscriber")
	}

	if stream == "internal" {
		stream = postgresSubscriber.config.InternalStream
	}

	expected := subscribeCmd
	if strings.HasPrefix(stream, "-") {
		expected = unsubscribeCmd
		stream = strings.TrimPrefix(stream, "-")
	}

	for i := 0; i < 20; i++ {
		if postgresSubscriber.getEvent(stream) == expected {
			return nil
		}

		time.Sleep(25 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for subscription event: %s", stream)
}

func setupPostgresPubSubTest(t *testing.T) (*pgadapter.Config, *pgxpool.Pool) {
	t.Helper()

	config := pgadapter.NewConfig()
	config.URL = testPostgresPubSubURL()
	config.BroadcastNotifyChannel = "anycable_test_broadcasts"
	config.PubSubNotifyChannel = "anycable_test_pubsub"
	config.InternalStream = "__anycable_test_internal__"
	config.PollIntervalMilliseconds = 25
	config.CleanupIntervalSeconds = 3600

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	schema := "anycable_test_pubsub_" + suffix

	config.URL = testPostgresPubSubURLWithSearchPath(config.URL, schema)
	config.BroadcastsTable = schema + ".anycable_broadcasts"
	config.PubSubTable = schema + ".anycable_pubsub"
	config.StreamOffsetsTable = schema + ".anycable_stream_offsets"

	createPostgresPubSubTestSchema(t, testPostgresPubSubURL(), schema)

	pool, err := pgadapter.NewPool(context.Background(), &config)
	require.NoError(t, err)

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("Skipping Postgres tests: %v", err)
	}

	return &config, pool
}

func createPostgresPubSubTestSchema(t *testing.T, rawURL string, schema string) {
	t.Helper()

	config := pgadapter.NewConfig()
	config.URL = rawURL

	pool, err := pgadapter.NewPool(context.Background(), &config)
	require.NoError(t, err)
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("Skipping Postgres tests: %v", err)
	}

	quotedSchema, err := pgadapter.QuoteIdentifier(schema)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), fmt.Sprintf("CREATE SCHEMA %s", quotedSchema))
	require.NoError(t, err)
}

func testPostgresPubSubURL() string {
	if url := os.Getenv("ANYCABLE_POSTGRES_TEST_URL"); url != "" {
		return url
	}

	return "postgres://localhost:5432/postgres?sslmode=disable"
}

func testPostgresPubSubURLWithSearchPath(rawURL string, schema string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()

	return parsed.String()
}

func dropPostgresPubSubTestTables(t *testing.T, pool *pgxpool.Pool, config *pgadapter.Config) {
	t.Helper()

	if schema, ok := postgresPubSubTestSchema(config.PubSubTable); ok {
		quotedSchema, err := pgadapter.QuoteIdentifier(schema)
		require.NoError(t, err)
		_, err = pool.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", quotedSchema))
		require.NoError(t, err)
		return
	}

	broadcastsTable, err := pgadapter.QuoteTableName(config.BroadcastsTable)
	require.NoError(t, err)
	pubsubTable, err := pgadapter.QuoteTableName(config.PubSubTable)
	require.NoError(t, err)
	offsetsTable, err := pgadapter.QuoteTableName(config.StreamOffsetsTable)
	require.NoError(t, err)

	_, err = pool.Exec(context.Background(), fmt.Sprintf(`
DROP TABLE IF EXISTS %s;
DROP TABLE IF EXISTS %s;
DROP TABLE IF EXISTS %s;
`, broadcastsTable, pubsubTable, offsetsTable))
	require.NoError(t, err)
}

func postgresPubSubTestSchema(table string) (string, bool) {
	schema, _, ok := strings.Cut(table, ".")
	if !ok || schema == "" {
		return "", false
	}

	return schema, true
}
