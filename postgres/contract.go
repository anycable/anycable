package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// BroadcastsTriggerName is required on the broadcast queue table.
	BroadcastsTriggerName = "anycable_broadcasts_notify_insert"
	// PubSubTriggerName is required on the pub/sub fan-out table.
	PubSubTriggerName = "anycable_pubsub_notify_insert"
)

// ColumnSpec describes a required column in the external signalling contract.
type ColumnSpec struct {
	Name    string
	Types   []string
	NotNull bool
}

type columnInfo struct {
	typ     string
	notNull bool
}

// NewPool validates the URL and creates a connection pool. Callers still run
// ValidateContract after this so that a reachable database cannot silently run
// with an incompatible schema.
func NewPool(ctx context.Context, config *Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(config.URL)
	if err != nil {
		return nil, err
	}

	return pgxpool.NewWithConfig(ctx, poolConfig)
}

// ValidateContract verifies the database contract expected by the Postgres
// adapters. The Go process never creates or migrates this schema; applications
// must install it ahead of time.
func ValidateContract(ctx context.Context, pool *pgxpool.Pool, config *Config) error {
	if err := ValidateIdentifiers(config); err != nil {
		return err
	}

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres connection check failed: %w", err)
	}

	if !config.ValidateContract {
		return nil
	}

	if err := validateContractVersion(ctx, pool, config); err != nil {
		return err
	}

	if err := validateColumns(ctx, pool, config.ContractTable, []ColumnSpec{
		{Name: "name", Types: []string{"text"}, NotNull: true},
		{Name: "version", Types: []string{"integer"}, NotNull: true},
		{Name: "created_at", Types: []string{"timestamp with time zone"}, NotNull: true},
	}); err != nil {
		return err
	}

	if err := validateColumns(ctx, pool, config.BroadcastsTable, []ColumnSpec{
		{Name: "id", Types: []string{"bigint"}, NotNull: true},
		{Name: "payload", Types: []string{"text"}, NotNull: true},
		{Name: "claimed_by", Types: []string{"text"}, NotNull: false},
		{Name: "claimed_at", Types: []string{"timestamp with time zone"}, NotNull: false},
		{Name: "attempts", Types: []string{"integer"}, NotNull: true},
		{Name: "last_error", Types: []string{"text"}, NotNull: false},
		{Name: "created_at", Types: []string{"timestamp with time zone"}, NotNull: true},
	}); err != nil {
		return err
	}

	if err := validateColumns(ctx, pool, config.PubSubTable, []ColumnSpec{
		{Name: "id", Types: []string{"bigint"}, NotNull: true},
		{Name: "stream", Types: []string{"text"}, NotNull: true},
		{Name: "payload", Types: []string{"text"}, NotNull: true},
		{Name: "created_at", Types: []string{"timestamp with time zone"}, NotNull: true},
	}); err != nil {
		return err
	}

	if err := validateTrigger(ctx, pool, config.BroadcastsTable, BroadcastsTriggerName); err != nil {
		return err
	}

	if err := validateTrigger(ctx, pool, config.PubSubTable, PubSubTriggerName); err != nil {
		return err
	}

	return nil
}

// ValidateIdentifiers rejects empty or unsafe dynamic identifiers before they
// are interpolated into SQL.
func ValidateIdentifiers(config *Config) error {
	if err := validateIdentifier("notify channel", config.NotifyChannel); err != nil {
		return err
	}

	if err := validateIdentifier("internal stream", config.InternalStream); err != nil {
		return err
	}

	if err := validateTableIdentifier("broadcasts table", config.BroadcastsTable); err != nil {
		return err
	}

	if err := validateTableIdentifier("pub/sub table", config.PubSubTable); err != nil {
		return err
	}

	if err := validateTableIdentifier("contract table", config.ContractTable); err != nil {
		return err
	}

	return nil
}

// QuoteIdentifier returns a safely quoted PostgreSQL identifier.
func QuoteIdentifier(name string) (string, error) {
	if err := validateIdentifier("identifier", name); err != nil {
		return "", err
	}

	return pgx.Identifier{name}.Sanitize(), nil
}

// QuoteTableName returns a safely quoted table name. It accepts either a bare
// table name or schema-qualified name.
func QuoteTableName(name string) (string, error) {
	if err := validateTableIdentifier("table", name); err != nil {
		return "", err
	}

	parts := strings.Split(name, ".")
	return pgx.Identifier(parts).Sanitize(), nil
}

func validateContractVersion(ctx context.Context, pool *pgxpool.Pool, config *Config) error {
	table, err := QuoteTableName(config.ContractTable)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("SELECT version FROM %s WHERE name = $1", table)

	var version int
	if err := pool.QueryRow(ctx, query, ContractName).Scan(&version); err != nil {
		return fmt.Errorf("postgres signalling contract %q is not installed in %s: %w", ContractName, config.ContractTable, err)
	}

	if version != ContractVersion {
		return fmt.Errorf("postgres signalling contract version mismatch: expected %d, got %d", ContractVersion, version)
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

func validateTrigger(ctx context.Context, pool *pgxpool.Pool, table string, trigger string) error {
	var exists bool
	err := pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM pg_catalog.pg_trigger
  WHERE tgrelid = to_regclass($1)
    AND tgname = $2
    AND NOT tgisinternal
)`, table, trigger).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to inspect trigger %s on %s: %w", trigger, table, err)
	}

	if !exists {
		return fmt.Errorf("postgres signalling table %s is missing trigger %s", table, trigger)
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

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}

	return false
}
