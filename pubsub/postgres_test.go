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
	config.NotifyChannel = "anycable_test_signals"
	config.InternalStream = "__anycable_test_internal__"
	config.PollIntervalMilliseconds = 25
	config.CleanupIntervalSeconds = 3600

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	config.ContractTable = "anycable_contracts_" + suffix
	config.BroadcastsTable = "anycable_broadcasts_" + suffix
	config.PubSubTable = "anycable_pubsub_" + suffix

	pool, err := pgadapter.NewPool(context.Background(), &config)
	require.NoError(t, err)

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("Skipping Postgres tests: %v", err)
	}

	installPostgresPubSubTestTables(t, pool, &config)

	return &config, pool
}

func testPostgresPubSubURL() string {
	if url := os.Getenv("ANYCABLE_POSTGRES_TEST_URL"); url != "" {
		return url
	}

	return "postgres://localhost:5432/postgres?sslmode=disable"
}

func installPostgresPubSubTestTables(t *testing.T, pool *pgxpool.Pool, config *pgadapter.Config) {
	t.Helper()

	// Keep this schema in sync with the Rails generator migration so pub/sub is
	// exercised against the same table/trigger contract applications install.
	contractTable, err := pgadapter.QuoteTableName(config.ContractTable)
	require.NoError(t, err)
	broadcastsTable, err := pgadapter.QuoteTableName(config.BroadcastsTable)
	require.NoError(t, err)
	pubsubTable, err := pgadapter.QuoteTableName(config.PubSubTable)
	require.NoError(t, err)
	functionName, err := pgadapter.QuoteIdentifier("anycable_test_notify_" + strings.ReplaceAll(config.BroadcastsTable, "anycable_broadcasts_", ""))
	require.NoError(t, err)
	broadcastsTrigger, err := pgadapter.QuoteIdentifier(pgadapter.BroadcastsTriggerName)
	require.NoError(t, err)
	pubsubTrigger, err := pgadapter.QuoteIdentifier(pgadapter.PubSubTriggerName)
	require.NoError(t, err)
	channelLiteral := strings.ReplaceAll(config.NotifyChannel, "'", "''")

	sql := fmt.Sprintf(`
CREATE TABLE %s (
  name text PRIMARY KEY,
  version integer NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO %s (name, version)
VALUES ('%s', %d);

CREATE TABLE %s (
  id bigserial PRIMARY KEY,
  payload text NOT NULL,
  claimed_by text,
  claimed_at timestamptz,
  attempts integer NOT NULL DEFAULT 0,
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ON %s (claimed_at, id) WHERE claimed_at IS NOT NULL;
CREATE INDEX ON %s (attempts, id);

CREATE TABLE %s (
  id bigserial PRIMARY KEY,
  stream text NOT NULL,
  payload text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ON %s (stream, id);
CREATE INDEX ON %s (created_at);

CREATE FUNCTION %s()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM pg_notify('%s', json_build_object('v', 1, 'table', TG_TABLE_NAME, 'id', NEW.id)::text);
  RETURN NEW;
END;
$$;

CREATE TRIGGER %s
AFTER INSERT ON %s
FOR EACH ROW EXECUTE FUNCTION %s();

CREATE TRIGGER %s
AFTER INSERT ON %s
FOR EACH ROW EXECUTE FUNCTION %s();
`, contractTable, contractTable, pgadapter.ContractName, pgadapter.ContractVersion, broadcastsTable, broadcastsTable, broadcastsTable, pubsubTable, pubsubTable, pubsubTable, functionName, channelLiteral, broadcastsTrigger, broadcastsTable, functionName, pubsubTrigger, pubsubTable, functionName)

	_, err = pool.Exec(context.Background(), sql)
	require.NoError(t, err)
}

func dropPostgresPubSubTestTables(t *testing.T, pool *pgxpool.Pool, config *pgadapter.Config) {
	t.Helper()

	contractTable, err := pgadapter.QuoteTableName(config.ContractTable)
	require.NoError(t, err)
	broadcastsTable, err := pgadapter.QuoteTableName(config.BroadcastsTable)
	require.NoError(t, err)
	pubsubTable, err := pgadapter.QuoteTableName(config.PubSubTable)
	require.NoError(t, err)
	functionName, err := pgadapter.QuoteIdentifier("anycable_test_notify_" + strings.ReplaceAll(config.BroadcastsTable, "anycable_broadcasts_", ""))
	require.NoError(t, err)

	_, err = pool.Exec(context.Background(), fmt.Sprintf(`
DROP TABLE IF EXISTS %s;
DROP TABLE IF EXISTS %s;
DROP TABLE IF EXISTS %s;
DROP FUNCTION IF EXISTS %s();
`, broadcastsTable, pubsubTable, contractTable, functionName))
	require.NoError(t, err)
}
