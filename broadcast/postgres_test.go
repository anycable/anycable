package broadcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
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

	offset, err := insertPostgresBroadcast(pool, config, map[string]string{"stream": "any_test", "data": "broken"})
	require.NoError(t, err)

	table, err := pgadapter.QuoteTableName(config.BroadcastsTable)
	require.NoError(t, err)

	var claimedBy string
	var claimedAtPresent bool
	var attempts int
	var lastError string
	var exhaustedAtPresent bool

	require.Eventually(t, func() bool {
		err := pool.QueryRow(
			context.Background(),
			fmt.Sprintf("SELECT claimed_by, claimed_at IS NOT NULL, attempts, last_error, exhausted_at IS NOT NULL FROM %s WHERE stream = $1 AND \"offset\" = $2", table),
			"any_test",
			offset,
		).Scan(&claimedBy, &claimedAtPresent, &attempts, &lastError, &exhaustedAtPresent)

		return err == nil &&
			claimedBy == config.NodeID() &&
			claimedAtPresent &&
			attempts == 1 &&
			lastError == "boom" &&
			exhaustedAtPresent
	}, time.Second, 25*time.Millisecond)
}

func TestPostgresBroadcasterPollsBatchAcrossStreams(t *testing.T) {
	config, pool := setupPostgresBroadcastTest(t)
	defer pool.Close()
	defer dropPostgresBroadcastTestTables(t, pool, config)

	config.BatchSize = 2
	require.NoError(t, pgadapter.EnsureSchema(context.Background(), pool, config))

	handler := &postgresRecordingHandler{}
	broadcaster := newPollingPostgresBroadcaster(handler, config, pool)

	require.NoError(t, publishPostgresBroadcast(pool, config, map[string]string{"stream": "alpha", "data": "a1"}))
	require.NoError(t, publishPostgresBroadcast(pool, config, map[string]string{"stream": "alpha", "data": "a2"}))
	require.NoError(t, publishPostgresBroadcast(pool, config, map[string]string{"stream": "beta", "data": "b1"}))

	count, err := broadcaster.pollOnce()
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.ElementsMatch(t, []string{"a1", "b1"}, handler.data())

	count, err = broadcaster.pollOnce()
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.ElementsMatch(t, []string{"a1", "a2", "b1"}, handler.data())
}

func TestPostgresBroadcasterExhaustedPolicyControlsSameStreamProgress(t *testing.T) {
	for _, tc := range []struct {
		name            string
		policy          string
		secondPollCount int
		expectedData    []string
	}{
		{
			name:            "skip exhausted row",
			policy:          pgadapter.ExhaustedBroadcastPolicySkip,
			secondPollCount: 1,
			expectedData:    []string{"broken", "later"},
		},
		{
			name:            "block behind exhausted row",
			policy:          pgadapter.ExhaustedBroadcastPolicyBlock,
			secondPollCount: 0,
			expectedData:    []string{"broken"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, pool := setupPostgresBroadcastTest(t)
			defer pool.Close()
			defer dropPostgresBroadcastTestTables(t, pool, config)

			config.BatchSize = 10
			config.MaxAttempts = 1
			config.ExhaustedBroadcastPolicy = tc.policy
			require.NoError(t, pgadapter.EnsureSchema(context.Background(), pool, config))

			handler := &postgresRecordingHandler{
				failures: map[string]error{"broken": errors.New("boom")},
			}
			broadcaster := newPollingPostgresBroadcaster(handler, config, pool)

			require.NoError(t, publishPostgresBroadcast(pool, config, map[string]string{"stream": "ordered", "data": "broken"}))
			require.NoError(t, publishPostgresBroadcast(pool, config, map[string]string{"stream": "ordered", "data": "later"}))

			count, err := broadcaster.pollOnce()
			require.NoError(t, err)
			assert.Equal(t, 1, count)

			count, err = broadcaster.pollOnce()
			require.NoError(t, err)
			assert.Equal(t, tc.secondPollCount, count)
			assert.Equal(t, tc.expectedData, handler.data())
		})
	}
}

func TestPostgresBroadcasterFinalAttemptTimeoutUnblocksSkipPolicy(t *testing.T) {
	config, pool := setupPostgresBroadcastTest(t)
	defer pool.Close()
	defer dropPostgresBroadcastTestTables(t, pool, config)

	config.BatchSize = 10
	config.ClaimTimeoutSeconds = 1
	config.MaxAttempts = 1
	config.ExhaustedBroadcastPolicy = pgadapter.ExhaustedBroadcastPolicySkip
	require.NoError(t, pgadapter.EnsureSchema(context.Background(), pool, config))

	firstOffset, err := insertPostgresBroadcast(pool, config, map[string]string{"stream": "ordered", "data": "timed-out"})
	require.NoError(t, err)
	require.NoError(t, publishPostgresBroadcast(pool, config, map[string]string{"stream": "ordered", "data": "later"}))

	table, err := pgadapter.QuoteTableName(config.BroadcastsTable)
	require.NoError(t, err)
	_, err = pool.Exec(
		context.Background(),
		fmt.Sprintf("UPDATE %s SET claimed_by = $3, claimed_at = now() - interval '5 seconds', attempts = $4 WHERE stream = $1 AND \"offset\" = $2", table),
		"ordered",
		firstOffset,
		config.NodeID(),
		config.AttemptsLimit(),
	)
	require.NoError(t, err)

	handler := &postgresRecordingHandler{}
	broadcaster := newPollingPostgresBroadcaster(handler, config, pool)

	count, err := broadcaster.pollOnce()
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, []string{"later"}, handler.data())

	var exhaustedAtPresent bool
	var lastError string
	err = pool.QueryRow(
		context.Background(),
		fmt.Sprintf("SELECT exhausted_at IS NOT NULL, last_error FROM %s WHERE stream = $1 AND \"offset\" = $2", table),
		"ordered",
		firstOffset,
	).Scan(&exhaustedAtPresent, &lastError)
	require.NoError(t, err)
	assert.True(t, exhaustedAtPresent)
	assert.Equal(t, "claim timed out on final attempt", lastError)
}

func TestPostgresBroadcasterCleanupRespectsExhaustedPolicy(t *testing.T) {
	for _, tc := range []struct {
		name       string
		policy     string
		expectRows int
	}{
		{
			name:       "skip removes expired exhausted rows",
			policy:     pgadapter.ExhaustedBroadcastPolicySkip,
			expectRows: 0,
		},
		{
			name:       "block leaves exhausted rows for operator action",
			policy:     pgadapter.ExhaustedBroadcastPolicyBlock,
			expectRows: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, pool := setupPostgresBroadcastTest(t)
			defer pool.Close()
			defer dropPostgresBroadcastTestTables(t, pool, config)

			config.ExhaustedBroadcastPolicy = tc.policy
			config.RetentionTTLSeconds = 1
			require.NoError(t, pgadapter.EnsureSchema(context.Background(), pool, config))

			offset, err := insertPostgresBroadcast(pool, config, map[string]string{"stream": "cleanup", "data": "expired"})
			require.NoError(t, err)

			table, err := pgadapter.QuoteTableName(config.BroadcastsTable)
			require.NoError(t, err)
			_, err = pool.Exec(
				context.Background(),
				fmt.Sprintf("UPDATE %s SET claimed_by = $3, claimed_at = now() - interval '1 hour', attempts = $4, last_error = $5, exhausted_at = now() - interval '1 hour' WHERE stream = $1 AND \"offset\" = $2", table),
				"cleanup",
				offset,
				config.NodeID(),
				config.AttemptsLimit(),
				"expired",
			)
			require.NoError(t, err)

			broadcaster := newPollingPostgresBroadcaster(&postgresRecordingHandler{}, config, pool)
			require.NoError(t, broadcaster.cleanup())

			var rows int
			err = pool.QueryRow(context.Background(), fmt.Sprintf("SELECT count(*) FROM %s WHERE stream = $1", table), "cleanup").Scan(&rows)
			require.NoError(t, err)
			assert.Equal(t, tc.expectRows, rows)
		})
	}
}

func setupPostgresBroadcastTest(t *testing.T) (*pgadapter.Config, *pgxpool.Pool) {
	t.Helper()

	config := pgadapter.NewConfig()
	config.URL = testPostgresBroadcastURL()
	config.BroadcastNotifyChannel = "anycable_test_broadcasts"
	config.PubSubNotifyChannel = "anycable_test_pubsub"
	config.InternalStream = "__anycable_test_internal__"
	config.ClaimID = "postgres-test"
	config.PollIntervalMilliseconds = 25
	config.CleanupIntervalSeconds = 3600

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	schema := "anycable_test_broadcast_" + suffix

	config.URL = testPostgresBroadcastURLWithSearchPath(config.URL, schema)
	config.BroadcastsTable = schema + ".anycable_broadcasts"
	config.PubSubTable = schema + ".anycable_pubsub"
	config.StreamOffsetsTable = schema + ".anycable_stream_offsets"

	createPostgresBroadcastTestSchema(t, testPostgresBroadcastURL(), schema)

	pool, err := pgadapter.NewPool(context.Background(), &config)
	require.NoError(t, err)

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("Skipping Postgres tests: %v", err)
	}

	return &config, pool
}

func createPostgresBroadcastTestSchema(t *testing.T, rawURL string, schema string) {
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

func newPollingPostgresBroadcaster(handler Handler, config *pgadapter.Config, pool *pgxpool.Pool) *PostgresBroadcaster {
	broadcaster := NewPostgresBroadcaster(handler, config, slog.Default())
	broadcaster.pool = pool
	broadcaster.ctx = context.Background()
	return broadcaster
}

func testPostgresBroadcastURL() string {
	if url := os.Getenv("ANYCABLE_POSTGRES_TEST_URL"); url != "" {
		return url
	}

	return "postgres://localhost:5432/postgres?sslmode=disable"
}

func testPostgresBroadcastURLWithSearchPath(rawURL string, schema string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()

	return parsed.String()
}

func publishPostgresBroadcast(pool *pgxpool.Pool, config *pgadapter.Config, payload map[string]string) error {
	_, err := insertPostgresBroadcast(pool, config, payload)
	return err
}

func insertPostgresBroadcast(pool *pgxpool.Pool, config *pgadapter.Config, payload map[string]string) (int64, error) {
	var id int64
	err := pool.QueryRow(context.Background(), "SELECT anycable_publish($1, $2, '{}')", payload["stream"], string(utils.ToJSON(payload))).Scan(&id)
	return id, err
}

func dropPostgresBroadcastTestTables(t *testing.T, pool *pgxpool.Pool, config *pgadapter.Config) {
	t.Helper()

	if schema, ok := postgresBroadcastTestSchema(config.BroadcastsTable); ok {
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

func postgresBroadcastTestSchema(table string) (string, bool) {
	schema, _, ok := strings.Cut(table, ".")
	if !ok || schema == "" {
		return "", false
	}

	return schema, true
}

type postgresRecordingHandler struct {
	failures map[string]error
	handled  []map[string]string
}

func (h *postgresRecordingHandler) HandleBroadcast(data []byte) error {
	var msg map[string]string
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}

	h.handled = append(h.handled, msg)

	if err, ok := h.failures[msg["data"]]; ok {
		return err
	}

	return nil
}

func (h *postgresRecordingHandler) HandlePubSub(data []byte) {
}

func (h *postgresRecordingHandler) data() []string {
	data := make([]string, 0, len(h.handled))
	for _, msg := range h.handled {
		data = append(data, msg["data"])
	}
	return data
}
