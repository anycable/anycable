package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_ToToml(t *testing.T) {
	config := NewConfig()
	config.URL = "postgres://example"
	config.NotifyChannel = "signals"
	config.InternalStream = "internal"
	config.BroadcastsTable = "broadcasts"
	config.PubSubTable = "pubsub"
	config.ContractTable = "contracts"
	config.PollIntervalMilliseconds = 25
	config.BatchSize = 5
	config.ClaimTimeoutSeconds = 3
	config.MaxAttempts = 2
	config.RetentionTTLSeconds = 60
	config.CleanupIntervalSeconds = 10
	config.StartupMaxAttempts = 3

	tomlStr := config.ToToml()

	assert.Contains(t, tomlStr, "url = \"postgres://example\"")
	assert.Contains(t, tomlStr, "notify_channel = \"signals\"")
	assert.Contains(t, tomlStr, "internal_stream = \"internal\"")
	assert.Contains(t, tomlStr, "broadcasts_table = \"broadcasts\"")
	assert.Contains(t, tomlStr, "pubsub_table = \"pubsub\"")
	assert.Contains(t, tomlStr, "startup_max_attempts = 3")

	config2 := NewConfig()

	_, err := toml.Decode(tomlStr, &config2)
	require.NoError(t, err)

	assert.Equal(t, config, config2)
}

func TestStartupWithRetry(t *testing.T) {
	t.Run("retries until success", func(t *testing.T) {
		config := NewConfig()
		config.StartupMaxAttempts = 3
		attempts := 0

		err := startupWithRetryDelay(context.Background(), &config, slog.Default(), "test", func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return fmt.Errorf("not ready")
			}

			return nil
		}, func(int) time.Duration {
			return time.Millisecond
		})

		require.NoError(t, err)
		assert.Equal(t, 3, attempts)
	})

	t.Run("fails after max attempts", func(t *testing.T) {
		config := NewConfig()
		config.StartupMaxAttempts = 2
		attempts := 0

		err := startupWithRetryDelay(context.Background(), &config, slog.Default(), "test", func(ctx context.Context) error {
			attempts++
			return fmt.Errorf("still down")
		}, func(int) time.Duration {
			return time.Millisecond
		})

		require.Error(t, err)
		assert.Equal(t, 2, attempts)
		assert.Contains(t, err.Error(), "postgres test startup failed after 2 attempt(s)")
	})
}

func TestValidateContract(t *testing.T) {
	ctx := context.Background()
	config, pool := setupContractTest(t)
	defer pool.Close()
	defer dropContractTestTables(t, pool, config)

	require.NoError(t, ValidateContract(ctx, pool, config))

	trigger, err := QuoteIdentifier(BroadcastsTriggerName)
	require.NoError(t, err)
	broadcastsTable, err := QuoteTableName(config.BroadcastsTable)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, fmt.Sprintf("ALTER TABLE %s DISABLE TRIGGER %s", broadcastsTable, trigger))
	require.NoError(t, err)

	err = ValidateContract(ctx, pool, config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enabled INSERT trigger")

	_, err = pool.Exec(ctx, fmt.Sprintf("ALTER TABLE %s ENABLE TRIGGER %s", broadcastsTable, trigger))
	require.NoError(t, err)
	require.NoError(t, ValidateContract(ctx, pool, config))

	_, err = pool.Exec(ctx, fmt.Sprintf("DROP TRIGGER %s ON %s", trigger, broadcastsTable))
	require.NoError(t, err)

	err = ValidateContract(ctx, pool, config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), BroadcastsTriggerName)
}

func setupContractTest(t *testing.T) (*Config, *pgxpool.Pool) {
	t.Helper()

	config := NewConfig()
	config.URL = testPostgresURL()
	config.NotifyChannel = "anycable_test_signals"
	config.InternalStream = "__anycable_test_internal__"
	config.PollIntervalMilliseconds = 25
	config.CleanupIntervalSeconds = 3600

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	config.ContractTable = "anycable_contracts_" + suffix
	config.BroadcastsTable = "anycable_broadcasts_" + suffix
	config.PubSubTable = "anycable_pubsub_" + suffix

	pool, err := NewPool(context.Background(), &config)
	require.NoError(t, err)

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("Skipping Postgres tests: %v", err)
	}

	installContractTestTables(t, pool, &config)

	return &config, pool
}

func testPostgresURL() string {
	if url := os.Getenv("ANYCABLE_POSTGRES_TEST_URL"); url != "" {
		return url
	}

	return "postgres://localhost:5432/postgres?sslmode=disable"
}

func installContractTestTables(t *testing.T, pool *pgxpool.Pool, config *Config) {
	t.Helper()

	// Keep this schema in sync with the Rails generator migration; the contract
	// validator is intentionally tested against the real table/trigger shape.
	ctx := context.Background()

	contractTable, err := QuoteTableName(config.ContractTable)
	require.NoError(t, err)
	broadcastsTable, err := QuoteTableName(config.BroadcastsTable)
	require.NoError(t, err)
	pubsubTable, err := QuoteTableName(config.PubSubTable)
	require.NoError(t, err)
	functionName, err := QuoteIdentifier("anycable_test_notify_" + strings.ReplaceAll(config.BroadcastsTable, "anycable_broadcasts_", ""))
	require.NoError(t, err)
	broadcastsTrigger, err := QuoteIdentifier(BroadcastsTriggerName)
	require.NoError(t, err)
	pubsubTrigger, err := QuoteIdentifier(PubSubTriggerName)
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
`, contractTable, contractTable, ContractName, ContractVersion, broadcastsTable, broadcastsTable, broadcastsTable, pubsubTable, pubsubTable, pubsubTable, functionName, channelLiteral, broadcastsTrigger, broadcastsTable, functionName, pubsubTrigger, pubsubTable, functionName)

	_, err = pool.Exec(ctx, sql)
	require.NoError(t, err)
}

func dropContractTestTables(t *testing.T, pool *pgxpool.Pool, config *Config) {
	t.Helper()

	ctx := context.Background()
	contractTable, err := QuoteTableName(config.ContractTable)
	require.NoError(t, err)
	broadcastsTable, err := QuoteTableName(config.BroadcastsTable)
	require.NoError(t, err)
	pubsubTable, err := QuoteTableName(config.PubSubTable)
	require.NoError(t, err)
	functionName, err := QuoteIdentifier("anycable_test_notify_" + strings.ReplaceAll(config.BroadcastsTable, "anycable_broadcasts_", ""))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, fmt.Sprintf(`
DROP TABLE IF EXISTS %s;
DROP TABLE IF EXISTS %s;
DROP TABLE IF EXISTS %s;
DROP FUNCTION IF EXISTS %s();
`, broadcastsTable, pubsubTable, contractTable, functionName))
	require.NoError(t, err)
}
