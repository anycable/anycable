package postgres

import (
	"fmt"
	"strings"
	"time"
)

const (
	ExhaustedBroadcastPolicySkip  = "skip"
	ExhaustedBroadcastPolicyBlock = "block"
)

// Config contains settings shared by the Postgres broadcaster and pub/sub
// subscriber. Both adapters use the same schema and separate NOTIFY channels.
type Config struct {
	// Postgres connection URL.
	URL string `toml:"url"`
	// Postgres NOTIFY channel used to wake app-to-server broadcast polling.
	BroadcastNotifyChannel string `toml:"broadcast_notify_channel"`
	// Postgres NOTIFY channel used to wake node-to-node pub/sub polling.
	PubSubNotifyChannel string `toml:"pubsub_notify_channel"`
	// Stream used for internal remote commands between AnyCable nodes.
	InternalStream string `toml:"internal_stream"`
	// Diagnostic owner stored on claimed broadcast rows.
	ClaimID string `toml:"claim_id"`
	// Table containing app-to-AnyCable broadcast messages.
	BroadcastsTable string `toml:"broadcasts_table"`
	// Table containing node-to-node pub/sub messages.
	PubSubTable string `toml:"pubsub_table"`
	// Table containing latest offsets per logical scope and stream.
	StreamOffsetsTable string `toml:"stream_offsets_table"`
	// Poll interval used as a correctness fallback when notifications are missed.
	PollIntervalMilliseconds int `toml:"poll_interval_milliseconds"`
	// Max number of rows to process in one batch.
	BatchSize int `toml:"batch_size"`
	// Seconds before an unfinished broadcast claim may be retried by another node.
	ClaimTimeoutSeconds int `toml:"claim_timeout_seconds"`
	// Max number of failed processing attempts before a broadcast is no longer retried.
	MaxAttempts int `toml:"max_attempts"`
	// Policy for exhausted broadcast rows: skip or block.
	ExhaustedBroadcastPolicy string `toml:"exhausted_broadcast_policy"`
	// Seconds to keep old pub/sub rows before cleanup.
	RetentionTTLSeconds int64 `toml:"retention_ttl"`
	// How often to run cleanup.
	CleanupIntervalSeconds int64 `toml:"cleanup_interval"`
	// Max attempts to connect and validate the schema during startup.
	StartupMaxAttempts int `toml:"startup_max_attempts"`
	// Create or actualize the Postgres signalling schema on startup.
	EnsureSchema bool `toml:"ensure_schema"`
}

// NewConfig returns default Postgres signalling settings.
func NewConfig() Config {
	return Config{
		URL:                      "postgres://localhost:5432/postgres?sslmode=disable",
		BroadcastNotifyChannel:   "anycable_broadcasts",
		PubSubNotifyChannel:      "anycable_pubsub",
		InternalStream:           "__anycable_internal__",
		BroadcastsTable:          "anycable_broadcasts",
		PubSubTable:              "anycable_pubsub",
		StreamOffsetsTable:       "anycable_stream_offsets",
		PollIntervalMilliseconds: 500,
		BatchSize:                100,
		ClaimTimeoutSeconds:      30,
		MaxAttempts:              5,
		ExhaustedBroadcastPolicy: ExhaustedBroadcastPolicySkip,
		RetentionTTLSeconds:      300,
		CleanupIntervalSeconds:   60,
		StartupMaxAttempts:       5,
		EnsureSchema:             true,
	}
}

// PollIntervalMS returns the poll fallback interval in milliseconds.
func (c Config) PollIntervalMS() int {
	if c.PollIntervalMilliseconds <= 0 {
		return 500
	}

	return c.PollIntervalMilliseconds
}

// PollInterval returns the poll fallback interval as a duration.
func (c Config) PollInterval() time.Duration {
	return time.Duration(c.PollIntervalMS()) * time.Millisecond
}

// BatchLimit returns the number of rows fetched per polling pass.
func (c Config) BatchLimit() int {
	if c.BatchSize <= 0 {
		return 100
	}

	return c.BatchSize
}

// ClaimTimeout returns the number of seconds before a broadcast claim expires.
func (c Config) ClaimTimeout() int {
	if c.ClaimTimeoutSeconds <= 0 {
		return 30
	}

	return c.ClaimTimeoutSeconds
}

// AttemptsLimit returns the number of failed attempts allowed for a broadcast.
func (c Config) AttemptsLimit() int {
	if c.MaxAttempts <= 0 {
		return 5
	}

	return c.MaxAttempts
}

func (c Config) ExhaustedPolicy() string {
	if c.ExhaustedBroadcastPolicy == ExhaustedBroadcastPolicyBlock {
		return ExhaustedBroadcastPolicyBlock
	}

	return ExhaustedBroadcastPolicySkip
}

// RetentionTTL returns the retention window in seconds.
func (c Config) RetentionTTL() int64 {
	if c.RetentionTTLSeconds <= 0 {
		return 300
	}

	return c.RetentionTTLSeconds
}

// RetentionDuration returns the retention window as a duration.
func (c Config) RetentionDuration() time.Duration {
	return time.Duration(c.RetentionTTL()) * time.Second
}

// CleanupInterval returns the cleanup cadence in seconds.
func (c Config) CleanupInterval() int64 {
	if c.CleanupIntervalSeconds <= 0 {
		return 60
	}

	return c.CleanupIntervalSeconds
}

// CleanupDuration returns the cleanup cadence as a duration.
func (c Config) CleanupDuration() time.Duration {
	return time.Duration(c.CleanupInterval()) * time.Second
}

// StartupAttempts returns the number of startup connection/schema attempts.
func (c Config) StartupAttempts() int {
	if c.StartupMaxAttempts <= 0 {
		return 5
	}

	return c.StartupMaxAttempts
}

// NodeID returns the diagnostic claim owner recorded on broadcast rows.
func (c Config) NodeID() string {
	if c.ClaimID == "" {
		return "anycable-go"
	}

	return c.ClaimID
}

// ToToml renders the Postgres configuration section.
func (c Config) ToToml() string {
	var result strings.Builder

	result.WriteString("# Postgres connection URL for AnyCable signalling\n")
	result.WriteString(fmt.Sprintf("url = \"%s\"\n", c.URL))

	result.WriteString("# Postgres NOTIFY channel used to wake app-to-server broadcast polling\n")
	result.WriteString(fmt.Sprintf("broadcast_notify_channel = \"%s\"\n", c.BroadcastNotifyChannel))

	result.WriteString("# Postgres NOTIFY channel used to wake node-to-node pub/sub polling\n")
	result.WriteString(fmt.Sprintf("pubsub_notify_channel = \"%s\"\n", c.PubSubNotifyChannel))

	result.WriteString("# Stream used for internal remote commands between AnyCable nodes\n")
	result.WriteString(fmt.Sprintf("internal_stream = \"%s\"\n", c.InternalStream))

	if c.ClaimID != "" {
		result.WriteString("# Diagnostic owner stored on claimed broadcast rows\n")
		result.WriteString(fmt.Sprintf("claim_id = \"%s\"\n", c.ClaimID))
	}

	result.WriteString("# Broadcast queue table\n")
	result.WriteString(fmt.Sprintf("broadcasts_table = \"%s\"\n", c.BroadcastsTable))

	result.WriteString("# Pub/sub fan-out table\n")
	result.WriteString(fmt.Sprintf("pubsub_table = \"%s\"\n", c.PubSubTable))

	result.WriteString("# Stream offset metadata table\n")
	result.WriteString(fmt.Sprintf("stream_offsets_table = \"%s\"\n", c.StreamOffsetsTable))

	result.WriteString("# Poll fallback interval in milliseconds\n")
	result.WriteString(fmt.Sprintf("poll_interval_milliseconds = %d\n", c.PollIntervalMS()))

	result.WriteString("# Max number of rows to process in one batch\n")
	result.WriteString(fmt.Sprintf("batch_size = %d\n", c.BatchLimit()))

	result.WriteString("# Seconds before an unfinished broadcast claim may be retried\n")
	result.WriteString(fmt.Sprintf("claim_timeout_seconds = %d\n", c.ClaimTimeout()))

	result.WriteString("# Max number of failed attempts before a broadcast is no longer retried\n")
	result.WriteString(fmt.Sprintf("max_attempts = %d\n", c.AttemptsLimit()))

	result.WriteString("# Policy for exhausted broadcast rows: skip or block\n")
	result.WriteString(fmt.Sprintf("exhausted_broadcast_policy = \"%s\"\n", c.ExhaustedPolicy()))

	result.WriteString("# Seconds to keep old pub/sub rows before cleanup\n")
	result.WriteString(fmt.Sprintf("retention_ttl = %d\n", c.RetentionTTL()))

	result.WriteString("# Cleanup interval in seconds\n")
	result.WriteString(fmt.Sprintf("cleanup_interval = %d\n", c.CleanupInterval()))

	result.WriteString("# Max startup attempts for connection and schema validation\n")
	result.WriteString(fmt.Sprintf("startup_max_attempts = %d\n", c.StartupAttempts()))

	result.WriteString("# Create or actualize the Postgres signalling schema on startup\n")
	if c.EnsureSchema {
		result.WriteString("ensure_schema = true\n")
	} else {
		result.WriteString("# ensure_schema = false\n")
	}

	result.WriteString("\n")

	return result.String()
}
