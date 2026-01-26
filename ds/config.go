package ds

import (
	"fmt"
	"strings"
)

// Durable Streams configuration
type Config struct {
	Enabled bool `toml:"enabled"`
	// Path is the URL path to handle HTTP requests
	Path string `toml:"path"`
	// Skip authentication
	SkipAuth bool `toml:"skip_auth"`
	// Poll interval  for live=poll mode (in seconds)
	PollInterval int `toml:"poll_interval"`
	// Max time to live for a SSE connection (in seconds)
	SSETTL         int    `toml:"sse_ttl"`
	AllowedOrigins string `toml:"-"`
}

// NewConfig creates a new Config with default values.
func NewConfig() Config {
	return Config{
		Enabled:      false,
		Path:         "/ds",
		PollInterval: 10,
		SSETTL:       60,
	}
}

// ToToml converts the Config struct to a TOML string representation
func (c Config) ToToml() string {
	var result strings.Builder

	result.WriteString("# Enable Durable Streams support\n")
	if c.Enabled {
		result.WriteString("enabled = true\n")
	} else {
		result.WriteString("# enabled = true\n")
	}

	result.WriteString("# Skip authentication (only authorize per-stream access)\n")
	if c.SkipAuth {
		result.WriteString("skip_auth = true\n")
	} else {
		result.WriteString("# skip_auth = true\n")
	}

	result.WriteString("# Durable Streams mount path\n")
	result.WriteString(fmt.Sprintf("path = \"%s\"\n", c.Path))

	result.WriteString("# Poll interval for live=poll mode (in seconds)\n")
	result.WriteString(fmt.Sprintf("poll_interval = %d\n", c.PollInterval))

	result.WriteString("# Max time to live for a SSE connection (in seconds)\n")
	result.WriteString(fmt.Sprintf("sse_ttl = %d\n", c.SSETTL))

	result.WriteString("\n")

	return result.String()
}
