package sse

import (
	"fmt"
	"strings"
)

const (
	defaultMaxBodySize = 65536 // 64 kB
)

// Server-sent events configuration
type Config struct {
	Enabled bool `toml:"enabled"`
	// Path is the URL path to handle SSE requests
	Path string `toml:"path"`
	// List of allowed origins for CORS requests
	// We inherit it from the ws.Config
	AllowedOrigins string
}

// NewConfig creates a new Config with default values.
func NewConfig() Config {
	return Config{
		Enabled: false,
		Path:    "/events",
	}
}

// ToToml converts the Config struct to a TOML string representation
func (c Config) ToToml() string {
	var result strings.Builder

	result.WriteString("# Enable Server-sent events support\n")
	if c.Enabled {
		result.WriteString("enabled = true\n")
	} else {
		result.WriteString("# enabled = true\n")
	}

	result.WriteString("# Server-sent events endpoint path\n")
	result.WriteString(fmt.Sprintf("path = \"%s\"\n", c.Path))

	result.WriteString("\n")

	return result.String()
}
