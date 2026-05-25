package pubsub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

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
	config.BroadcastsTable = "anycable_broadcasts_" + suffix
	config.PubSubTable = "anycable_pubsub_" + suffix
	config.StreamOffsetsTable = "anycable_stream_offsets_" + suffix

	pool, err := pgadapter.NewPool(context.Background(), &config)
	require.NoError(t, err)

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("Skipping Postgres tests: %v", err)
	}

	return &config, pool
}

func testPostgresPubSubURL() string {
	if url := os.Getenv("ANYCABLE_POSTGRES_TEST_URL"); url != "" {
		return url
	}

	return "postgres://localhost:5432/postgres?sslmode=disable"
}

func dropPostgresPubSubTestTables(t *testing.T, pool *pgxpool.Pool, config *pgadapter.Config) {
	t.Helper()

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
DROP FUNCTION IF EXISTS anycable_publish(text, text, text);
DROP FUNCTION IF EXISTS anycable_remote_command(text, text);
`, broadcastsTable, pubsubTable, offsetsTable))
	require.NoError(t, err)
}
