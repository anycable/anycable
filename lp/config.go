package lp

import (
	"fmt"
	"strings"
)

const (
	defaultMaxBodySize = 65536 // 64 kB
)

// Long-polling configuration
type Config struct {
	Enabled bool `toml:"enabled"`
	// Path is the URL path to handle long-polling requests
	Path string `toml:"path"`
	// MaxBodySize is the maximum size of the request body in bytes.
	MaxBodySize int `toml:"max_body_size"`
	// FlushInterval is the duration before flushing buffered data to the client in milliseconds.
	FlushInterval int `toml:"flush_interval"`
	// PollInterval is the default polling duration in seconds
	PollInterval int `toml:"poll_interval"`
	// KeepaliveTimeout is the duration for how long to consider the virtual connection to be alive between poll requests (in seconds)
	KeepaliveTimeout int `toml:"keepalive_timeout"`
	// List of allowed origins for CORS requests
	// We inherit it from the server.Config
	AllowedOrigins string `toml:"-"`
}

// NewConfig creates a new Config with default values.
func NewConfig() Config {
	return Config{
		Enabled:          false,
		Path:             "/lp",
		MaxBodySize:      defaultMaxBodySize,
		FlushInterval:    500,
		PollInterval:     15,
		KeepaliveTimeout: 5,
	}
}

func (c Config) ToToml() string {
	var result strings.Builder

	prefix := ""

	if c.Enabled {
		result.WriteString("enabled = true\n")
	} else {
		prefix = "# "
		result.WriteString("# enabled = true\n")
	}

	result.WriteString("# Long-polling endpoint path\n")
	result.WriteString(fmt.Sprintf("%spath = \"%s\"\n", prefix, c.Path))

	result.WriteString("# Poll interval\n")
	result.WriteString(fmt.Sprintf("%spoll_interval = %d\n", prefix, c.PollInterval))

	result.WriteString("# Keepalive timeout\n")
	result.WriteString(fmt.Sprintf("%skeepalive_timeout = %d\n", prefix, c.KeepaliveTimeout))

	result.WriteString("# Flush interval\n")
	result.WriteString(fmt.Sprintf("%sflush_interval = %d\n", prefix, c.FlushInterval))

	result.WriteString("# Maximum body size\n")
	result.WriteString(fmt.Sprintf("%smax_body_size = %d\n", prefix, c.MaxBodySize))

	result.WriteString("\n")

	return result.String()
}
