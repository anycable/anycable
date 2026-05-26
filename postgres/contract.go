package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// BroadcastsTriggerName is the trigger name used for app-to-server broadcast
	// notifications.
	BroadcastsTriggerName = "anycable_broadcasts_notify_insert"
	// PubSubTriggerName is the trigger name used for node-to-node pub/sub
	// notifications.
	PubSubTriggerName = "anycable_pubsub_notify_insert"

	broadcastScope = "broadcast"
	pubSubScope    = "pubsub"
)

// ColumnSpec describes the required shape of a Postgres signalling table column.
type ColumnSpec struct {
	Name                 string
	Types                []string
	NotNull              bool
	AutoValue            bool
	DefaultExprFragments []string
}

type columnInfo struct {
	typ         string
	notNull     bool
	identity    bool
	defaultExpr string
}

// NewPool opens a pgx connection pool from the Postgres signalling config.
func NewPool(ctx context.Context, config *Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(config.URL)
	if err != nil {
		return nil, err
	}

	return pgxpool.NewWithConfig(ctx, poolConfig)
}

// EnsureSchema verifies connectivity, optionally actualizes the signalling
// schema, and validates the resulting contract.
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

// ValidateSchema checks that all required tables, indexes, triggers, and SQL
// functions match the Postgres signalling contract.
func ValidateSchema(ctx context.Context, pool *pgxpool.Pool, config *Config) error {
	if err := validateColumns(ctx, pool, config.StreamOffsetsTable, []ColumnSpec{
		{Name: "scope", Types: []string{"text"}, NotNull: true},
		{Name: "stream", Types: []string{"text"}, NotNull: true},
		{Name: "offset", Types: []string{"bigint"}, NotNull: true},
		{Name: "updated_at", Types: []string{"timestamp with time zone"}, NotNull: true, DefaultExprFragments: []string{"now()"}},
	}); err != nil {
		return err
	}

	if err := validateColumns(ctx, pool, config.BroadcastsTable, []ColumnSpec{
		{Name: "id", Types: []string{"bigint"}, NotNull: true, AutoValue: true},
		{Name: "stream", Types: []string{"text"}, NotNull: true},
		{Name: "offset", Types: []string{"bigint"}, NotNull: true},
		{Name: "payload", Types: []string{"text"}, NotNull: true},
		{Name: "meta", Types: []string{"text"}, NotNull: true},
		{Name: "claimed_by", Types: []string{"text"}, NotNull: false},
		{Name: "claimed_at", Types: []string{"timestamp with time zone"}, NotNull: false},
		{Name: "attempts", Types: []string{"integer"}, NotNull: true, DefaultExprFragments: []string{"0"}},
		{Name: "last_error", Types: []string{"text"}, NotNull: false},
		{Name: "exhausted_at", Types: []string{"timestamp with time zone"}, NotNull: false},
		{Name: "created_at", Types: []string{"timestamp with time zone"}, NotNull: true, DefaultExprFragments: []string{"now()"}},
	}); err != nil {
		return err
	}

	if err := validateColumns(ctx, pool, config.PubSubTable, []ColumnSpec{
		{Name: "id", Types: []string{"bigint"}, NotNull: true, AutoValue: true},
		{Name: "stream", Types: []string{"text"}, NotNull: true},
		{Name: "offset", Types: []string{"bigint"}, NotNull: true},
		{Name: "payload", Types: []string{"text"}, NotNull: true},
		{Name: "meta", Types: []string{"text"}, NotNull: true},
		{Name: "created_at", Types: []string{"timestamp with time zone"}, NotNull: true, DefaultExprFragments: []string{"now()"}},
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

	if err := validateTrigger(ctx, pool, config.BroadcastsTable, BroadcastsTriggerName, triggerFunctionName(config.BroadcastsTable, broadcastScope)); err != nil {
		return err
	}

	if err := validateTrigger(ctx, pool, config.PubSubTable, PubSubTriggerName, triggerFunctionName(config.PubSubTable, pubSubScope)); err != nil {
		return err
	}

	if err := validateTriggerFunction(ctx, pool, triggerFunctionName(config.BroadcastsTable, broadcastScope), config.BroadcastNotifyChannel); err != nil {
		return err
	}

	if err := validateTriggerFunction(ctx, pool, triggerFunctionName(config.PubSubTable, pubSubScope), config.PubSubNotifyChannel); err != nil {
		return err
	}

	if err := validateFunction(ctx, pool, "anycable_publish", "text, text, text", 1); err != nil {
		return err
	}

	if err := validateFunction(ctx, pool, "anycable_remote_command", "text, text", 1); err != nil {
		return err
	}

	offsetsTable, err := QuoteTableName(config.StreamOffsetsTable)
	if err != nil {
		return err
	}
	broadcastsTable, err := QuoteTableName(config.BroadcastsTable)
	if err != nil {
		return err
	}

	if err := validateFunctionBody(ctx, pool, "anycable_publish", [][]string{
		{"INSERT INTO"},
		{config.StreamOffsetsTable, offsetsTable},
		{config.BroadcastsTable, broadcastsTable},
		{"ON CONFLICT (scope, stream)"},
		{`RETURNING "offset"`},
		{"target_stream"},
		{"payload"},
		{"meta"},
	}); err != nil {
		return err
	}

	if err := validateFunctionBody(ctx, pool, "anycable_remote_command", [][]string{
		{"INSERT INTO"},
		{config.StreamOffsetsTable, offsetsTable},
		{config.BroadcastsTable, broadcastsTable},
		{"ON CONFLICT (scope, stream)"},
		{`RETURNING "offset"`},
		{config.InternalStream, sqlLiteral(config.InternalStream)},
		{"payload"},
		{"meta"},
	}); err != nil {
		return err
	}

	return nil
}

// ValidateIdentifiers rejects unsafe or unsupported configured identifiers.
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

// QuoteIdentifier validates and quotes a single Postgres identifier.
func QuoteIdentifier(name string) (string, error) {
	if err := validateIdentifier("identifier", name); err != nil {
		return "", err
	}

	return pgx.Identifier{name}.Sanitize(), nil
}

// QuoteTableName validates and quotes a table name, optionally schema-qualified.
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

ALTER TABLE %s ADD COLUMN IF NOT EXISTS scope text;
ALTER TABLE %s ADD COLUMN IF NOT EXISTS stream text;
ALTER TABLE %s ADD COLUMN IF NOT EXISTS "offset" bigint DEFAULT 0;
ALTER TABLE %s ADD COLUMN IF NOT EXISTS updated_at timestamptz DEFAULT now();
UPDATE %s SET scope = '' WHERE scope IS NULL;
UPDATE %s SET stream = '' WHERE stream IS NULL;
UPDATE %s SET "offset" = 0 WHERE "offset" IS NULL;
UPDATE %s SET updated_at = now() WHERE updated_at IS NULL;
ALTER TABLE %s ALTER COLUMN scope SET NOT NULL;
ALTER TABLE %s ALTER COLUMN stream SET NOT NULL;
ALTER TABLE %s ALTER COLUMN "offset" SET DEFAULT 0;
ALTER TABLE %s ALTER COLUMN "offset" SET NOT NULL;
ALTER TABLE %s ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE %s ALTER COLUMN updated_at SET NOT NULL;

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
ALTER TABLE %s ADD COLUMN IF NOT EXISTS payload text;
ALTER TABLE %s ADD COLUMN IF NOT EXISTS meta text NOT NULL DEFAULT '{}';
ALTER TABLE %s ADD COLUMN IF NOT EXISTS claimed_by text;
ALTER TABLE %s ADD COLUMN IF NOT EXISTS claimed_at timestamptz;
ALTER TABLE %s ADD COLUMN IF NOT EXISTS attempts integer DEFAULT 0;
ALTER TABLE %s ADD COLUMN IF NOT EXISTS last_error text;
ALTER TABLE %s ADD COLUMN IF NOT EXISTS exhausted_at timestamptz;
ALTER TABLE %s ADD COLUMN IF NOT EXISTS created_at timestamptz DEFAULT now();
UPDATE %s SET stream = '' WHERE stream IS NULL;
UPDATE %s SET "offset" = id WHERE "offset" IS NULL;
UPDATE %s SET payload = '' WHERE payload IS NULL;
UPDATE %s SET meta = '{}' WHERE meta IS NULL;
UPDATE %s SET attempts = 0 WHERE attempts IS NULL;
UPDATE %s SET created_at = now() WHERE created_at IS NULL;
ALTER TABLE %s ALTER COLUMN stream SET NOT NULL;
ALTER TABLE %s ALTER COLUMN "offset" SET NOT NULL;
ALTER TABLE %s ALTER COLUMN payload SET NOT NULL;
ALTER TABLE %s ALTER COLUMN meta SET DEFAULT '{}';
ALTER TABLE %s ALTER COLUMN meta SET NOT NULL;
ALTER TABLE %s ALTER COLUMN attempts SET DEFAULT 0;
ALTER TABLE %s ALTER COLUMN attempts SET NOT NULL;
ALTER TABLE %s ALTER COLUMN created_at SET DEFAULT now();
ALTER TABLE %s ALTER COLUMN created_at SET NOT NULL;

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
ALTER TABLE %s ADD COLUMN IF NOT EXISTS stream text;
ALTER TABLE %s ADD COLUMN IF NOT EXISTS payload text;
ALTER TABLE %s ADD COLUMN IF NOT EXISTS meta text NOT NULL DEFAULT '{}';
ALTER TABLE %s ADD COLUMN IF NOT EXISTS created_at timestamptz DEFAULT now();
UPDATE %s SET "offset" = id WHERE "offset" IS NULL;
UPDATE %s SET stream = '' WHERE stream IS NULL;
UPDATE %s SET payload = '' WHERE payload IS NULL;
UPDATE %s SET meta = '{}' WHERE meta IS NULL;
UPDATE %s SET created_at = now() WHERE created_at IS NULL;
ALTER TABLE %s ALTER COLUMN "offset" SET NOT NULL;
ALTER TABLE %s ALTER COLUMN stream SET NOT NULL;
ALTER TABLE %s ALTER COLUMN payload SET NOT NULL;
ALTER TABLE %s ALTER COLUMN meta SET DEFAULT '{}';
ALTER TABLE %s ALTER COLUMN meta SET NOT NULL;
ALTER TABLE %s ALTER COLUMN created_at SET DEFAULT now();
ALTER TABLE %s ALTER COLUMN created_at SET NOT NULL;

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
		offsetsTable, offsetsTable, offsetsTable, offsetsTable,
		offsetsTable, offsetsTable, offsetsTable, offsetsTable,
		offsetsTable, offsetsTable, offsetsTable, offsetsTable, offsetsTable, offsetsTable,
		broadcastsTable,
		broadcastsTable, broadcastsTable, broadcastsTable, broadcastsTable, broadcastsTable,
		broadcastsTable, broadcastsTable, broadcastsTable, broadcastsTable, broadcastsTable,
		broadcastsTable, broadcastsTable, broadcastsTable, broadcastsTable, broadcastsTable, broadcastsTable,
		broadcastsTable, broadcastsTable, broadcastsTable, broadcastsTable, broadcastsTable, broadcastsTable, broadcastsTable, broadcastsTable, broadcastsTable,
		broadcastOffsetIndex, broadcastsTable, broadcastClaimIndex, broadcastsTable, broadcastCleanupIndex, broadcastsTable,
		pubsubTable,
		pubsubTable, pubsubTable, pubsubTable, pubsubTable, pubsubTable,
		pubsubTable, pubsubTable, pubsubTable, pubsubTable, pubsubTable,
		pubsubTable, pubsubTable, pubsubTable, pubsubTable, pubsubTable, pubsubTable, pubsubTable,
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
SELECT
  a.attname,
  pg_catalog.format_type(a.atttypid, a.atttypmod),
  a.attnotnull,
  a.attidentity <> '',
  COALESCE(pg_get_expr(d.adbin, d.adrelid), '')
FROM pg_catalog.pg_attribute a
LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
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
		var name, typ, defaultExpr string
		var notNull bool
		var identity bool
		if err := rows.Scan(&name, &typ, &notNull, &identity, &defaultExpr); err != nil {
			return fmt.Errorf("failed to inspect %s: %w", table, err)
		}
		columns[name] = columnInfo{typ: typ, notNull: notNull, identity: identity, defaultExpr: defaultExpr}
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
		if spec.AutoValue && !info.identity && !strings.Contains(info.defaultExpr, "nextval(") {
			return fmt.Errorf("postgres signalling table %s column %s must use an identity or sequence default", table, spec.Name)
		}
		if len(spec.DefaultExprFragments) > 0 && !matchesAnyFragment(info.defaultExpr, spec.DefaultExprFragments) {
			return fmt.Errorf("postgres signalling table %s column %s must define a default containing one of %s", table, spec.Name, strings.Join(spec.DefaultExprFragments, ", "))
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
    AND i.indpred IS NULL
    AND i.indexprs IS NULL
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

func matchesAnyFragment(value string, fragments []string) bool {
	for _, fragment := range fragments {
		if strings.Contains(value, fragment) {
			return true
		}
	}

	return false
}

func validateTrigger(ctx context.Context, pool *pgxpool.Pool, table string, trigger string, functionName string) error {
	var exists bool
	err := pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM pg_catalog.pg_trigger
  JOIN pg_catalog.pg_proc p ON p.oid = tgfoid
  WHERE tgrelid = to_regclass($1)
    AND tgname = $2
    AND p.proname = $3
    AND NOT tgisinternal
    AND tgenabled <> 'D'
    AND (tgtype::int & 1) = 1
    AND (tgtype::int & 2) = 0
    AND (tgtype::int & 4) = 4
)`, table, trigger, functionName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to inspect trigger %s on %s: %w", trigger, table, err)
	}
	if !exists {
		return fmt.Errorf("postgres signalling table %s is missing enabled AFTER INSERT row trigger %s using function %s", table, trigger, functionName)
	}
	return nil
}

func validateTriggerFunction(ctx context.Context, pool *pgxpool.Pool, name string, channel string) error {
	var exists bool
	err := pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM pg_catalog.pg_proc p
  JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
  JOIN pg_catalog.pg_language l ON l.oid = p.prolang
  WHERE p.proname = $1
    AND n.nspname = current_schema()
    AND p.pronargs = 0
    AND pg_catalog.format_type(p.prorettype, NULL) = 'trigger'
    AND l.lanname = 'plpgsql'
    AND position('pg_notify' in p.prosrc) > 0
    AND position($2 in p.prosrc) > 0
    AND position('json_build_object' in p.prosrc) > 0
    AND position('''v''' in p.prosrc) > 0
    AND p.prosrc ~ '''v''[[:space:]]*,[[:space:]]*1'
    AND position('''stream''' in p.prosrc) > 0
    AND position('NEW.stream' in p.prosrc) > 0
    AND position('''offset''' in p.prosrc) > 0
    AND position('NEW."offset"' in p.prosrc) > 0
)`, name, channel).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to inspect trigger function %s: %w", name, err)
	}
	if !exists {
		return fmt.Errorf("postgres signalling trigger function %s does not match expected NOTIFY payload contract", name)
	}
	return nil
}

func validateFunction(ctx context.Context, pool *pgxpool.Pool, name string, argTypes string, defaultCount int) error {
	var exists bool
	err := pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM pg_catalog.pg_proc p
  JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
  JOIN pg_catalog.pg_language l ON l.oid = p.prolang
  WHERE p.proname = $1
    AND n.nspname = current_schema()
    AND oidvectortypes(p.proargtypes) = $2
    AND p.pronargdefaults = $3
    AND pg_catalog.format_type(p.prorettype, NULL) = 'bigint'
    AND l.lanname = 'plpgsql'
)`, name, argTypes, defaultCount).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to inspect function %s: %w", name, err)
	}
	if !exists {
		return fmt.Errorf("postgres signalling function %s(%s) with %d default argument(s) returning bigint does not exist", name, argTypes, defaultCount)
	}
	return nil
}

func validateFunctionBody(ctx context.Context, pool *pgxpool.Pool, name string, requiredGroups [][]string) error {
	var source string
	err := pool.QueryRow(ctx, `
SELECT p.prosrc
FROM pg_catalog.pg_proc p
JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
WHERE p.proname = $1
  AND n.nspname = current_schema()
`, name).Scan(&source)
	if err != nil {
		return fmt.Errorf("failed to inspect function %s body: %w", name, err)
	}

	for _, alternatives := range requiredGroups {
		matched := false
		for _, fragment := range alternatives {
			if strings.Contains(source, fragment) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("postgres signalling function %s body does not match expected contract; missing one of %s", name, strings.Join(alternatives, ", "))
		}
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
