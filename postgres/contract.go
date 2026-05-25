package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	BroadcastsTriggerName = "anycable_broadcasts_notify_insert"
	PubSubTriggerName     = "anycable_pubsub_notify_insert"

	broadcastScope = "broadcast"
	pubSubScope    = "pubsub"
)

type ColumnSpec struct {
	Name    string
	Types   []string
	NotNull bool
}

type columnInfo struct {
	typ     string
	notNull bool
}

func NewPool(ctx context.Context, config *Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(config.URL)
	if err != nil {
		return nil, err
	}

	return pgxpool.NewWithConfig(ctx, poolConfig)
}

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, config *Config) error {
	if err := ValidateIdentifiers(config); err != nil {
		return err
	}

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres connection check failed: %w", err)
	}

	if config.EnsureSchema {
		if err := actualizeSchema(ctx, pool, config); err != nil {
			return err
		}
	}

	return ValidateSchema(ctx, pool, config)
}

func ValidateSchema(ctx context.Context, pool *pgxpool.Pool, config *Config) error {
	if err := validateColumns(ctx, pool, config.StreamOffsetsTable, []ColumnSpec{
		{Name: "scope", Types: []string{"text"}, NotNull: true},
		{Name: "stream", Types: []string{"text"}, NotNull: true},
		{Name: "offset", Types: []string{"bigint"}, NotNull: true},
		{Name: "updated_at", Types: []string{"timestamp with time zone"}, NotNull: true},
	}); err != nil {
		return err
	}

	if err := validateColumns(ctx, pool, config.BroadcastsTable, []ColumnSpec{
		{Name: "id", Types: []string{"bigint"}, NotNull: true},
		{Name: "stream", Types: []string{"text"}, NotNull: true},
		{Name: "offset", Types: []string{"bigint"}, NotNull: true},
		{Name: "payload", Types: []string{"text"}, NotNull: true},
		{Name: "meta", Types: []string{"text"}, NotNull: true},
		{Name: "claimed_by", Types: []string{"text"}, NotNull: false},
		{Name: "claimed_at", Types: []string{"timestamp with time zone"}, NotNull: false},
		{Name: "attempts", Types: []string{"integer"}, NotNull: true},
		{Name: "last_error", Types: []string{"text"}, NotNull: false},
		{Name: "exhausted_at", Types: []string{"timestamp with time zone"}, NotNull: false},
		{Name: "created_at", Types: []string{"timestamp with time zone"}, NotNull: true},
	}); err != nil {
		return err
	}

	if err := validateColumns(ctx, pool, config.PubSubTable, []ColumnSpec{
		{Name: "id", Types: []string{"bigint"}, NotNull: true},
		{Name: "stream", Types: []string{"text"}, NotNull: true},
		{Name: "offset", Types: []string{"bigint"}, NotNull: true},
		{Name: "payload", Types: []string{"text"}, NotNull: true},
		{Name: "meta", Types: []string{"text"}, NotNull: true},
		{Name: "created_at", Types: []string{"timestamp with time zone"}, NotNull: true},
	}); err != nil {
		return err
	}

	for _, check := range []struct {
		table   string
		columns []string
	}{
		{config.StreamOffsetsTable, []string{"scope", "stream"}},
		{config.BroadcastsTable, []string{"stream", "offset"}},
		{config.PubSubTable, []string{"stream", "offset"}},
	} {
		if err := validateUniqueIndex(ctx, pool, check.table, check.columns); err != nil {
			return err
		}
	}

	if err := validateTrigger(ctx, pool, config.BroadcastsTable, BroadcastsTriggerName); err != nil {
		return err
	}

	if err := validateTrigger(ctx, pool, config.PubSubTable, PubSubTriggerName); err != nil {
		return err
	}

	if err := validateFunction(ctx, pool, "anycable_publish", 3); err != nil {
		return err
	}

	if err := validateFunction(ctx, pool, "anycable_remote_command", 2); err != nil {
		return err
	}

	return nil
}

func ValidateIdentifiers(config *Config) error {
	for label, name := range map[string]string{
		"broadcast notify channel": config.BroadcastNotifyChannel,
		"pub/sub notify channel":   config.PubSubNotifyChannel,
		"internal stream":          config.InternalStream,
	} {
		if err := validateIdentifier(label, name); err != nil {
			return err
		}
	}

	for label, name := range map[string]string{
		"broadcasts table":     config.BroadcastsTable,
		"pub/sub table":        config.PubSubTable,
		"stream offsets table": config.StreamOffsetsTable,
	} {
		if err := validateTableIdentifier(label, name); err != nil {
			return err
		}
	}

	if config.ExhaustedBroadcastPolicy != "" &&
		config.ExhaustedBroadcastPolicy != ExhaustedBroadcastPolicySkip &&
		config.ExhaustedBroadcastPolicy != ExhaustedBroadcastPolicyBlock {
		return fmt.Errorf("invalid postgres exhausted broadcast policy %q", config.ExhaustedBroadcastPolicy)
	}

	return nil
}

func QuoteIdentifier(name string) (string, error) {
	if err := validateIdentifier("identifier", name); err != nil {
		return "", err
	}

	return pgx.Identifier{name}.Sanitize(), nil
}

func QuoteTableName(name string) (string, error) {
	if err := validateTableIdentifier("table", name); err != nil {
		return "", err
	}

	parts := strings.Split(name, ".")
	return pgx.Identifier(parts).Sanitize(), nil
}

func actualizeSchema(ctx context.Context, pool *pgxpool.Pool, config *Config) error {
	offsetsTable, err := QuoteTableName(config.StreamOffsetsTable)
	if err != nil {
		return err
	}
	broadcastsTable, err := QuoteTableName(config.BroadcastsTable)
	if err != nil {
		return err
	}
	pubsubTable, err := QuoteTableName(config.PubSubTable)
	if err != nil {
		return err
	}
	broadcastTriggerFn, err := QuoteIdentifier(triggerFunctionName(config.BroadcastsTable, broadcastScope))
	if err != nil {
		return err
	}
	pubsubTriggerFn, err := QuoteIdentifier(triggerFunctionName(config.PubSubTable, pubSubScope))
	if err != nil {
		return err
	}
	broadcastTrigger, err := QuoteIdentifier(BroadcastsTriggerName)
	if err != nil {
		return err
	}
	pubsubTrigger, err := QuoteIdentifier(PubSubTriggerName)
	if err != nil {
		return err
	}
	broadcastOffsetIndex, err := QuoteIdentifier(indexName(config.BroadcastsTable, "stream_offset_idx"))
	if err != nil {
		return err
	}
	broadcastClaimIndex, err := QuoteIdentifier(indexName(config.BroadcastsTable, "claim_idx"))
	if err != nil {
		return err
	}
	broadcastCleanupIndex, err := QuoteIdentifier(indexName(config.BroadcastsTable, "cleanup_idx"))
	if err != nil {
		return err
	}
	pubsubOffsetIndex, err := QuoteIdentifier(indexName(config.PubSubTable, "stream_offset_idx"))
	if err != nil {
		return err
	}
	pubsubCreatedIndex, err := QuoteIdentifier(indexName(config.PubSubTable, "created_at_idx"))
	if err != nil {
		return err
	}

	broadcastChannel := sqlLiteral(config.BroadcastNotifyChannel)
	pubsubChannel := sqlLiteral(config.PubSubNotifyChannel)
	internalStream := sqlLiteral(config.InternalStream)

	sql := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
  scope text NOT NULL,
  stream text NOT NULL,
  "offset" bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (scope, stream)
);

CREATE TABLE IF NOT EXISTS %s (
  id bigserial PRIMARY KEY,
  stream text NOT NULL,
  "offset" bigint NOT NULL,
  payload text NOT NULL,
  meta text NOT NULL DEFAULT '{}',
  claimed_by text,
  claimed_at timestamptz,
  attempts integer NOT NULL DEFAULT 0,
  last_error text,
  exhausted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE %s ADD COLUMN IF NOT EXISTS stream text;
ALTER TABLE %s ADD COLUMN IF NOT EXISTS "offset" bigint;
ALTER TABLE %s ADD COLUMN IF NOT EXISTS meta text NOT NULL DEFAULT '{}';
ALTER TABLE %s ADD COLUMN IF NOT EXISTS exhausted_at timestamptz;
UPDATE %s SET stream = '' WHERE stream IS NULL;
UPDATE %s SET "offset" = id WHERE "offset" IS NULL;
ALTER TABLE %s ALTER COLUMN stream SET NOT NULL;
ALTER TABLE %s ALTER COLUMN "offset" SET NOT NULL;
ALTER TABLE %s ALTER COLUMN meta SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (stream, "offset");
CREATE INDEX IF NOT EXISTS %s ON %s (claimed_at, stream, "offset") WHERE exhausted_at IS NULL;
CREATE INDEX IF NOT EXISTS %s ON %s (exhausted_at) WHERE exhausted_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS %s (
  id bigserial PRIMARY KEY,
  stream text NOT NULL,
  "offset" bigint NOT NULL,
  payload text NOT NULL,
  meta text NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE %s ADD COLUMN IF NOT EXISTS "offset" bigint;
ALTER TABLE %s ADD COLUMN IF NOT EXISTS meta text NOT NULL DEFAULT '{}';
UPDATE %s SET "offset" = id WHERE "offset" IS NULL;
ALTER TABLE %s ALTER COLUMN "offset" SET NOT NULL;
ALTER TABLE %s ALTER COLUMN meta SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (stream, "offset");
CREATE INDEX IF NOT EXISTS %s ON %s (created_at);

CREATE OR REPLACE FUNCTION %s()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM pg_notify(%s, json_build_object('v', 1, 'stream', NEW.stream, 'offset', NEW."offset")::text);
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS %s ON %s;
CREATE TRIGGER %s
AFTER INSERT ON %s
FOR EACH ROW EXECUTE FUNCTION %s();

CREATE OR REPLACE FUNCTION %s()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM pg_notify(%s, json_build_object('v', 1, 'stream', NEW.stream, 'offset', NEW."offset")::text);
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS %s ON %s;
CREATE TRIGGER %s
AFTER INSERT ON %s
FOR EACH ROW EXECUTE FUNCTION %s();

CREATE OR REPLACE FUNCTION anycable_publish(target_stream text, payload text, meta text DEFAULT '{}')
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  next_offset bigint;
BEGIN
  IF target_stream IS NULL OR target_stream = '' THEN
    RAISE EXCEPTION 'anycable_publish stream cannot be null or empty';
  END IF;
  IF target_stream = %s THEN
    RAISE EXCEPTION 'anycable_publish stream cannot use the configured internal stream';
  END IF;
  IF payload IS NULL THEN
    RAISE EXCEPTION 'anycable_publish payload cannot be null';
  END IF;
  IF meta IS NULL THEN
    RAISE EXCEPTION 'anycable_publish meta cannot be null';
  END IF;

  INSERT INTO %s AS offsets (scope, stream, "offset")
  VALUES ('broadcast', target_stream, 1)
  ON CONFLICT (scope, stream)
  DO UPDATE SET "offset" = offsets."offset" + 1,
                updated_at = now()
  RETURNING "offset" INTO next_offset;

  INSERT INTO %s (stream, "offset", payload, meta)
  VALUES (target_stream, next_offset, payload, meta);

  RETURN next_offset;
END;
$$;

CREATE OR REPLACE FUNCTION anycable_remote_command(payload text, meta text DEFAULT '{}')
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  next_offset bigint;
BEGIN
  IF %s IS NULL OR %s = '' THEN
    RAISE EXCEPTION 'anycable_remote_command internal stream cannot be null or empty';
  END IF;
  IF payload IS NULL THEN
    RAISE EXCEPTION 'anycable_remote_command payload cannot be null';
  END IF;
  IF meta IS NULL THEN
    RAISE EXCEPTION 'anycable_remote_command meta cannot be null';
  END IF;

  INSERT INTO %s AS offsets (scope, stream, "offset")
  VALUES ('broadcast', %s, 1)
  ON CONFLICT (scope, stream)
  DO UPDATE SET "offset" = offsets."offset" + 1,
                updated_at = now()
  RETURNING "offset" INTO next_offset;

  INSERT INTO %s (stream, "offset", payload, meta)
  VALUES (%s, next_offset, payload, meta);

  RETURN next_offset;
END;
$$;
`, offsetsTable,
		broadcastsTable,
		broadcastsTable, broadcastsTable, broadcastsTable, broadcastsTable,
		broadcastsTable, broadcastsTable, broadcastsTable, broadcastsTable, broadcastsTable,
		broadcastOffsetIndex, broadcastsTable, broadcastClaimIndex, broadcastsTable, broadcastCleanupIndex, broadcastsTable,
		pubsubTable,
		pubsubTable, pubsubTable, pubsubTable, pubsubTable, pubsubTable,
		pubsubOffsetIndex, pubsubTable, pubsubCreatedIndex, pubsubTable,
		broadcastTriggerFn, broadcastChannel,
		broadcastTrigger, broadcastsTable, broadcastTrigger, broadcastsTable, broadcastTriggerFn,
		pubsubTriggerFn, pubsubChannel,
		pubsubTrigger, pubsubTable, pubsubTrigger, pubsubTable, pubsubTriggerFn,
		internalStream,
		offsetsTable,
		broadcastsTable,
		internalStream, internalStream,
		offsetsTable, internalStream,
		broadcastsTable, internalStream)

	if _, err := pool.Exec(ctx, sql); err != nil {
		return fmt.Errorf("failed to ensure postgres signalling schema: %w", err)
	}

	return nil
}

func validateColumns(ctx context.Context, pool *pgxpool.Pool, table string, expected []ColumnSpec) error {
	rows, err := pool.Query(ctx, `
SELECT a.attname, pg_catalog.format_type(a.atttypid, a.atttypmod), a.attnotnull
FROM pg_catalog.pg_attribute a
WHERE a.attrelid = to_regclass($1)
  AND a.attnum > 0
  AND NOT a.attisdropped
`, table)
	if err != nil {
		return fmt.Errorf("failed to inspect %s: %w", table, err)
	}
	defer rows.Close()

	columns := map[string]columnInfo{}
	for rows.Next() {
		var name, typ string
		var notNull bool
		if err := rows.Scan(&name, &typ, &notNull); err != nil {
			return fmt.Errorf("failed to inspect %s: %w", table, err)
		}
		columns[name] = columnInfo{typ: typ, notNull: notNull}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to inspect %s: %w", table, err)
	}

	if len(columns) == 0 {
		return fmt.Errorf("postgres signalling table %s does not exist", table)
	}

	for _, spec := range expected {
		info, ok := columns[spec.Name]
		if !ok {
			return fmt.Errorf("postgres signalling table %s is missing required column %s", table, spec.Name)
		}
		if !containsString(spec.Types, info.typ) {
			return fmt.Errorf("postgres signalling table %s column %s has type %s; expected one of %s", table, spec.Name, info.typ, strings.Join(spec.Types, ", "))
		}
		if spec.NotNull && !info.notNull {
			return fmt.Errorf("postgres signalling table %s column %s must be NOT NULL", table, spec.Name)
		}
	}

	return nil
}

func validateUniqueIndex(ctx context.Context, pool *pgxpool.Pool, table string, columns []string) error {
	var exists bool
	err := pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM pg_catalog.pg_index i
  WHERE i.indrelid = to_regclass($1)
    AND i.indisvalid
    AND i.indisunique
    AND (
      SELECT array_agg(a.attname::text ORDER BY keys.ordinality)
      FROM unnest(i.indkey) WITH ORDINALITY AS keys(attnum, ordinality)
      JOIN pg_catalog.pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = keys.attnum
    ) = $2::text[]
)`, table, columns).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to inspect unique index on %s (%s): %w", table, strings.Join(columns, ", "), err)
	}
	if !exists {
		return fmt.Errorf("postgres signalling table %s is missing required unique index on %s", table, strings.Join(columns, ", "))
	}
	return nil
}

func validateTrigger(ctx context.Context, pool *pgxpool.Pool, table string, trigger string) error {
	var exists bool
	err := pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM pg_catalog.pg_trigger
  WHERE tgrelid = to_regclass($1)
    AND tgname = $2
    AND NOT tgisinternal
    AND tgenabled <> 'D'
    AND (tgtype::int & 4) = 4
)`, table, trigger).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to inspect trigger %s on %s: %w", trigger, table, err)
	}
	if !exists {
		return fmt.Errorf("postgres signalling table %s is missing enabled INSERT trigger %s", table, trigger)
	}
	return nil
}

func validateFunction(ctx context.Context, pool *pgxpool.Pool, name string, argCount int) error {
	var exists bool
	err := pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM pg_catalog.pg_proc p
  JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
  WHERE p.proname = $1
    AND n.nspname = current_schema()
    AND p.pronargs = $2
    AND pg_catalog.format_type(p.prorettype, NULL) = 'bigint'
)`, name, argCount).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to inspect function %s: %w", name, err)
	}
	if !exists {
		return fmt.Errorf("postgres signalling function %s with %d arguments returning bigint does not exist", name, argCount)
	}
	return nil
}

func validateTableIdentifier(label string, name string) error {
	parts := strings.Split(name, ".")
	if len(parts) > 2 {
		return fmt.Errorf("invalid postgres %s %q: expected table or schema.table", label, name)
	}
	for _, part := range parts {
		if err := validateIdentifier(label, part); err != nil {
			return err
		}
	}
	return nil
}

func validateIdentifier(label string, name string) error {
	if name == "" {
		return fmt.Errorf("postgres %s cannot be empty", label)
	}
	if strings.ContainsAny(name, "\x00\r\n") {
		return fmt.Errorf("postgres %s %q contains an invalid control character", label, name)
	}
	if strings.TrimSpace(name) != name {
		return fmt.Errorf("postgres %s %q cannot contain leading or trailing whitespace", label, name)
	}
	return nil
}

func triggerFunctionName(table string, scope string) string {
	return fmt.Sprintf("%s_%s_notify", strings.ReplaceAll(table, ".", "_"), scope)
}

func indexName(table string, suffix string) string {
	return fmt.Sprintf("%s_%s", strings.ReplaceAll(table, ".", "_"), suffix)
}

func sqlLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
