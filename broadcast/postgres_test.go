package broadcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/anycable/anycable-go/mocks"
	pgadapter "github.com/anycable/anycable-go/postgres"
	"github.com/anycable/anycable-go/utils"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPostgresBroadcaster(t *testing.T) {
	config, pool := setupPostgresBroadcastTest(t)
	defer pool.Close()
	defer dropPostgresBroadcastTestTables(t, pool, config)

	handler := &mocks.Handler{}
	errchan := make(chan error, 2)
	broadcasts := make(chan map[string]string, 10)

	handler.On(
		"HandleBroadcast",
		mock.Anything,
	).Run(func(args mock.Arguments) {
		data := args.Get(0).([]byte)
		var msg map[string]string
		json.Unmarshal(data, &msg) // nolint:errcheck

		broadcasts <- msg
	}).Return(nil)

	t.Run("Handles broadcasts", func(t *testing.T) {
		broadcaster := NewPostgresBroadcaster(handler, config, slog.Default())

		err := broadcaster.Start(errchan)
		require.NoError(t, err)
		defer broadcaster.Shutdown(context.Background()) // nolint:errcheck

		require.NoError(t, publishPostgresBroadcast(pool, config, map[string]string{"stream": "any_test", "data": "123_test"}))

		messages := drainBroadcasts(broadcasts)
		require.Equalf(t, 1, len(messages), "Expected 1 message, got %d", len(messages))

		assert.Equal(t, "any_test", messages[0]["stream"])
		assert.Equal(t, "123_test", messages[0]["data"])
	})

	t.Run("With multiple subscribers", func(t *testing.T) {
		broadcaster := NewPostgresBroadcaster(handler, config, slog.Default())
		err := broadcaster.Start(errchan)
		require.NoError(t, err)
		defer broadcaster.Shutdown(context.Background()) // nolint:errcheck

		otherConfig := *config
		otherConfig.ClaimID = "postgres-test-other"
		broadcaster2 := NewPostgresBroadcaster(handler, &otherConfig, slog.Default())
		err = broadcaster2.Start(errchan)
		require.NoError(t, err)
		defer broadcaster2.Shutdown(context.Background()) // nolint:errcheck

		require.NoError(t, publishPostgresBroadcast(pool, config, map[string]string{"stream": "any_test", "data": "123_test"}))
		require.NoError(t, publishPostgresBroadcast(pool, config, map[string]string{"stream": "any_test", "data": "124_test"}))
		require.NoError(t, publishPostgresBroadcast(pool, config, map[string]string{"stream": "any_test", "data": "125_test"}))

		messages := drainBroadcasts(broadcasts)
		require.Equalf(t, 3, len(messages), "Expected 3 messages, got %d", len(messages))
	})
}

func TestPostgresBroadcasterFinalFailureKeepsClaim(t *testing.T) {
	config, pool := setupPostgresBroadcastTest(t)
	defer pool.Close()
	defer dropPostgresBroadcastTestTables(t, pool, config)

	config.MaxAttempts = 1

	handler := &mocks.Handler{}
	handler.On("HandleBroadcast", mock.Anything).Return(errors.New("boom"))

	errchan := make(chan error, 2)
	broadcaster := NewPostgresBroadcaster(handler, config, slog.Default())
	err := broadcaster.Start(errchan)
	require.NoError(t, err)
	defer broadcaster.Shutdown(context.Background()) // nolint:errcheck

	id, err := insertPostgresBroadcast(pool, config, map[string]string{"stream": "any_test", "data": "broken"})
	require.NoError(t, err)

	table, err := pgadapter.QuoteTableName(config.BroadcastsTable)
	require.NoError(t, err)

	var claimedBy string
	var claimedAtPresent bool
	var attempts int
	var lastError string

	require.Eventually(t, func() bool {
		err := pool.QueryRow(
			context.Background(),
			fmt.Sprintf("SELECT claimed_by, claimed_at IS NOT NULL, attempts, last_error FROM %s WHERE id = $1", table),
			id,
		).Scan(&claimedBy, &claimedAtPresent, &attempts, &lastError)

		return err == nil &&
			claimedBy == config.NodeID() &&
			claimedAtPresent &&
			attempts == 1 &&
			lastError == "boom"
	}, time.Second, 25*time.Millisecond)
}

func setupPostgresBroadcastTest(t *testing.T) (*pgadapter.Config, *pgxpool.Pool) {
	t.Helper()

	config := pgadapter.NewConfig()
	config.URL = testPostgresBroadcastURL()
	config.NotifyChannel = "anycable_test_signals"
	config.InternalStream = "__anycable_test_internal__"
	config.ClaimID = "postgres-test"
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

	installPostgresBroadcastTestTables(t, pool, &config)

	return &config, pool
}

func testPostgresBroadcastURL() string {
	if url := os.Getenv("ANYCABLE_POSTGRES_TEST_URL"); url != "" {
		return url
	}

	return "postgres://localhost:5432/postgres?sslmode=disable"
}

func publishPostgresBroadcast(pool *pgxpool.Pool, config *pgadapter.Config, payload map[string]string) error {
	_, err := insertPostgresBroadcast(pool, config, payload)
	return err
}

func insertPostgresBroadcast(pool *pgxpool.Pool, config *pgadapter.Config, payload map[string]string) (int64, error) {
	table, err := pgadapter.QuoteTableName(config.BroadcastsTable)
	if err != nil {
		return 0, err
	}

	query := fmt.Sprintf("INSERT INTO %s (payload) VALUES ($1) RETURNING id", table)
	var id int64
	err = pool.QueryRow(context.Background(), query, string(utils.ToJSON(payload))).Scan(&id)
	return id, err
}

func installPostgresBroadcastTestTables(t *testing.T, pool *pgxpool.Pool, config *pgadapter.Config) {
	t.Helper()

	// Keep this schema in sync with the Rails generator migration so the
	// broadcaster runs against the same contract applications install.
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

func dropPostgresBroadcastTestTables(t *testing.T, pool *pgxpool.Pool, config *pgadapter.Config) {
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
