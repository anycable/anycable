package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
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
	config.BroadcastNotifyChannel = "broadcasts"
	config.PubSubNotifyChannel = "pubsub_signals"
	config.InternalStream = "internal"
	config.BroadcastsTable = "broadcasts"
	config.PubSubTable = "pubsub"
	config.StreamOffsetsTable = "offsets"
	config.PollIntervalMilliseconds = 25
	config.BatchSize = 5
	config.ClaimTimeoutSeconds = 3
	config.MaxAttempts = 2
	config.ExhaustedBroadcastPolicy = ExhaustedBroadcastPolicyBlock
	config.RetentionTTLSeconds = 60
	config.CleanupIntervalSeconds = 10
	config.StartupMaxAttempts = 3

	tomlStr := config.ToToml()

	assert.Contains(t, tomlStr, "url = \"postgres://example\"")
	assert.Contains(t, tomlStr, "broadcast_notify_channel = \"broadcasts\"")
	assert.Contains(t, tomlStr, "pubsub_notify_channel = \"pubsub_signals\"")
	assert.Contains(t, tomlStr, "internal_stream = \"internal\"")
	assert.Contains(t, tomlStr, "broadcasts_table = \"broadcasts\"")
	assert.Contains(t, tomlStr, "pubsub_table = \"pubsub\"")
	assert.Contains(t, tomlStr, "stream_offsets_table = \"offsets\"")
	assert.Contains(t, tomlStr, "exhausted_broadcast_policy = \"block\"")
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

func TestValidateIdentifiersRejectsInvalidExhaustedPolicy(t *testing.T) {
	config := NewConfig()
	config.ExhaustedBroadcastPolicy = "pause"

	err := ValidateIdentifiers(&config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid postgres exhausted broadcast policy")
}

func TestPostgresEnsureSchema(t *testing.T) {
	ctx := context.Background()
	config, pool := setupContractTest(t)
	defer pool.Close()
	defer dropContractTestTables(t, pool, config)

	require.NoError(t, EnsureSchema(ctx, pool, config))
	require.NoError(t, EnsureSchema(ctx, pool, config))

	offset, err := callPublish(ctx, pool, "any_test", "payload", "meta")
	require.NoError(t, err)
	assert.Equal(t, int64(1), offset)

	var storedPayload string
	var storedMeta string
	broadcastsTable, err := QuoteTableName(config.BroadcastsTable)
	require.NoError(t, err)
	err = pool.QueryRow(ctx, fmt.Sprintf("SELECT payload, meta FROM %s WHERE stream = $1 AND \"offset\" = $2", broadcastsTable), "any_test", offset).Scan(&storedPayload, &storedMeta)
	require.NoError(t, err)
	assert.Equal(t, "payload", storedPayload)
	assert.Equal(t, "meta", storedMeta)

	trigger, err := QuoteIdentifier(BroadcastsTriggerName)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, fmt.Sprintf("ALTER TABLE %s DISABLE TRIGGER %s", broadcastsTable, trigger))
	require.NoError(t, err)

	config.EnsureSchema = false
	err = EnsureSchema(ctx, pool, config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enabled AFTER INSERT row trigger")

	config.EnsureSchema = true
	require.NoError(t, EnsureSchema(ctx, pool, config))

	config.EnsureSchema = false
	_, err = pool.Exec(ctx, fmt.Sprintf("ALTER TABLE %s ENABLE TRIGGER %s", broadcastsTable, trigger))
	require.NoError(t, err)
	require.NoError(t, EnsureSchema(ctx, pool, config))

	_, err = pool.Exec(ctx, fmt.Sprintf("DROP TRIGGER %s ON %s", trigger, broadcastsTable))
	require.NoError(t, err)

	err = EnsureSchema(ctx, pool, config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), BroadcastsTriggerName)
}

func TestPostgresEnsureSchemaValidatesFunctionAndTriggerContracts(t *testing.T) {
	t.Run("function signature", func(t *testing.T) {
		ctx := context.Background()
		config, pool := setupContractTest(t)
		defer pool.Close()
		defer dropContractTestTables(t, pool, config)

		require.NoError(t, EnsureSchema(ctx, pool, config))

		_, err := pool.Exec(ctx, `
DROP FUNCTION anycable_publish(text, text, text);
CREATE FUNCTION anycable_publish(target_stream text, payload jsonb, meta text DEFAULT '{}')
RETURNS bigint
LANGUAGE plpgsql
AS $$
BEGIN
  RETURN 1;
END;
$$;
`)
		require.NoError(t, err)

		config.EnsureSchema = false
		err = EnsureSchema(ctx, pool, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "anycable_publish(text, text, text)")
	})

	t.Run("function body", func(t *testing.T) {
		ctx := context.Background()
		config, pool := setupContractTest(t)
		defer pool.Close()
		defer dropContractTestTables(t, pool, config)

		require.NoError(t, EnsureSchema(ctx, pool, config))

		_, err := pool.Exec(ctx, `
CREATE OR REPLACE FUNCTION anycable_publish(target_stream text, payload text, meta text DEFAULT '{}')
RETURNS bigint
LANGUAGE plpgsql
AS $$
BEGIN
  RETURN 1;
END;
$$;
`)
		require.NoError(t, err)

		config.EnsureSchema = false
		err = EnsureSchema(ctx, pool, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "body does not match expected contract")
	})

	t.Run("trigger function body", func(t *testing.T) {
		ctx := context.Background()
		config, pool := setupContractTest(t)
		defer pool.Close()
		defer dropContractTestTables(t, pool, config)

		require.NoError(t, EnsureSchema(ctx, pool, config))

		broadcastFn, err := QuoteIdentifier(triggerFunctionName(config.BroadcastsTable, broadcastScope))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, fmt.Sprintf(`
CREATE OR REPLACE FUNCTION %s()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM pg_notify('wrong_channel', '{}');
  RETURN NEW;
END;
$$;
`, broadcastFn))
		require.NoError(t, err)

		config.EnsureSchema = false
		err = EnsureSchema(ctx, pool, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "payload contract")
	})

	t.Run("trigger function identity", func(t *testing.T) {
		ctx := context.Background()
		config, pool := setupContractTest(t)
		defer pool.Close()
		defer dropContractTestTables(t, pool, config)

		require.NoError(t, EnsureSchema(ctx, pool, config))

		broadcastsTable, err := QuoteTableName(config.BroadcastsTable)
		require.NoError(t, err)
		broadcastTrigger, err := QuoteIdentifier(BroadcastsTriggerName)
		require.NoError(t, err)
		wrongFn, err := QuoteIdentifier(triggerFunctionName(config.BroadcastsTable, "wrong"))
		require.NoError(t, err)

		_, err = pool.Exec(ctx, fmt.Sprintf(`
CREATE FUNCTION %s()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RETURN NEW;
END;
$$;
DROP TRIGGER %s ON %s;
CREATE TRIGGER %s
AFTER INSERT ON %s
FOR EACH ROW EXECUTE FUNCTION %s();
`, wrongFn, broadcastTrigger, broadcastsTable, broadcastTrigger, broadcastsTable, wrongFn))
		require.NoError(t, err)

		config.EnsureSchema = false
		err = EnsureSchema(ctx, pool, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "using function")
	})
}

func TestPostgresEnsureSchemaActualizesPartialTables(t *testing.T) {
	ctx := context.Background()
	config, pool := setupContractTest(t)
	defer pool.Close()
	defer dropContractTestTables(t, pool, config)

	broadcastsTable, err := QuoteTableName(config.BroadcastsTable)
	require.NoError(t, err)
	pubsubTable, err := QuoteTableName(config.PubSubTable)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE %s (
  id bigserial PRIMARY KEY
);

CREATE TABLE %s (
  id bigserial PRIMARY KEY
);
`, broadcastsTable, pubsubTable))
	require.NoError(t, err)

	require.NoError(t, EnsureSchema(ctx, pool, config))
	require.NoError(t, ValidateSchema(ctx, pool, config))
}

func TestPostgresEnsureSchemaValidatesExternalTableContracts(t *testing.T) {
	t.Run("missing runtime default", func(t *testing.T) {
		ctx := context.Background()
		config, pool := setupContractTest(t)
		defer pool.Close()
		defer dropContractTestTables(t, pool, config)

		require.NoError(t, EnsureSchema(ctx, pool, config))

		broadcastsTable, err := QuoteTableName(config.BroadcastsTable)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN id DROP DEFAULT", broadcastsTable))
		require.NoError(t, err)

		config.EnsureSchema = false
		err = EnsureSchema(ctx, pool, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "column id must use an identity or sequence default")
	})

	t.Run("partial unique index", func(t *testing.T) {
		ctx := context.Background()
		config, pool := setupContractTest(t)
		defer pool.Close()
		defer dropContractTestTables(t, pool, config)

		require.NoError(t, EnsureSchema(ctx, pool, config))

		broadcastsTable, err := QuoteTableName(config.BroadcastsTable)
		require.NoError(t, err)
		broadcastOffsetIndex, err := QuoteIdentifier(indexName(config.BroadcastsTable, "stream_offset_idx"))
		require.NoError(t, err)

		_, err = pool.Exec(ctx, fmt.Sprintf(`
DROP INDEX %s;
CREATE UNIQUE INDEX %s ON %s (stream, "offset") WHERE stream <> 'excluded';
`, broadcastOffsetIndex, broadcastOffsetIndex, broadcastsTable))
		require.NoError(t, err)

		config.EnsureSchema = false
		err = EnsureSchema(ctx, pool, config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing required unique index")
	})
}

func setupContractTest(t *testing.T) (*Config, *pgxpool.Pool) {
	t.Helper()

	config := NewConfig()
	config.URL = testPostgresURL()
	config.BroadcastNotifyChannel = "anycable_test_broadcasts"
	config.PubSubNotifyChannel = "anycable_test_pubsub"
	config.InternalStream = "__anycable_test_internal__"
	config.PollIntervalMilliseconds = 25
	config.CleanupIntervalSeconds = 3600

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	schema := "anycable_test_" + suffix

	config.URL = testPostgresURLWithSearchPath(config.URL, schema)
	config.BroadcastsTable = schema + ".anycable_broadcasts"
	config.PubSubTable = schema + ".anycable_pubsub"
	config.StreamOffsetsTable = schema + ".anycable_stream_offsets"

	createContractTestSchema(t, testPostgresURL(), schema)

	pool, err := NewPool(context.Background(), &config)
	require.NoError(t, err)

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("Skipping Postgres tests: %v", err)
	}

	return &config, pool
}

func createContractTestSchema(t *testing.T, rawURL string, schema string) {
	t.Helper()

	config := NewConfig()
	config.URL = rawURL

	pool, err := NewPool(context.Background(), &config)
	require.NoError(t, err)
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("Skipping Postgres tests: %v", err)
	}

	quotedSchema, err := QuoteIdentifier(schema)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), fmt.Sprintf("CREATE SCHEMA %s", quotedSchema))
	require.NoError(t, err)
}

func testPostgresURL() string {
	if url := os.Getenv("ANYCABLE_POSTGRES_TEST_URL"); url != "" {
		return url
	}

	return "postgres://localhost:5432/postgres?sslmode=disable"
}

func testPostgresURLWithSearchPath(rawURL string, schema string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()

	return parsed.String()
}

func dropContractTestTables(t *testing.T, pool *pgxpool.Pool, config *Config) {
	t.Helper()

	ctx := context.Background()
	if schema, ok := contractTestSchema(config.BroadcastsTable); ok {
		quotedSchema, err := QuoteIdentifier(schema)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", quotedSchema))
		require.NoError(t, err)
		return
	}

	broadcastsTable, err := QuoteTableName(config.BroadcastsTable)
	require.NoError(t, err)
	pubsubTable, err := QuoteTableName(config.PubSubTable)
	require.NoError(t, err)
	offsetsTable, err := QuoteTableName(config.StreamOffsetsTable)
	require.NoError(t, err)
	broadcastFn, _ := QuoteIdentifier(triggerFunctionName(config.BroadcastsTable, broadcastScope))
	pubsubFn, _ := QuoteIdentifier(triggerFunctionName(config.PubSubTable, pubSubScope))
	wrongFn, _ := QuoteIdentifier(triggerFunctionName(config.BroadcastsTable, "wrong"))

	_, err = pool.Exec(ctx, fmt.Sprintf(`
DROP TABLE IF EXISTS %s;
DROP TABLE IF EXISTS %s;
DROP TABLE IF EXISTS %s;
DROP FUNCTION IF EXISTS %s();
DROP FUNCTION IF EXISTS %s();
DROP FUNCTION IF EXISTS %s();
`, broadcastsTable, pubsubTable, offsetsTable, broadcastFn, pubsubFn, wrongFn))
	require.NoError(t, err)
}

func contractTestSchema(table string) (string, bool) {
	schema, _, ok := strings.Cut(table, ".")
	if !ok || schema == "" {
		return "", false
	}

	return schema, true
}

func callPublish(ctx context.Context, pool *pgxpool.Pool, stream string, payload string, meta string) (int64, error) {
	var offset int64
	err := pool.QueryRow(ctx, "SELECT anycable_publish($1, $2, $3)", stream, payload, meta).Scan(&offset)
	return offset, err
}
