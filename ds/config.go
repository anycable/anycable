package ds

import (
	"fmt"
	"strings"
)

// Durable Streams configuration
type Config struct {
	Enabled bool `toml:"enabled"`
	// Path is the URL path to handle HTTP requests
	Path           string `toml:"path"`
	AllowedOrigins string `toml:"-"`
}

// NewConfig creates a new Config with default values.
func NewConfig() Config {
	return Config{
		Enabled: false,
		Path:    "/ds",
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

	result.WriteString("# Durable Streams mount path\n")
	result.WriteString(fmt.Sprintf("path = \"%s\"\n", c.Path))

	result.WriteString("\n")

	return result.String()
}
