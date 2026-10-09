package sse

import (
	"fmt"
	"strings"
)

const (
	defaultMaxBodySize  = 65536 // 64 kB
	DefaultPingInterval = 15
)

// Server-sent events configuration
type Config struct {
	Enabled bool `toml:"enabled"`
	// Path is the URL path to handle SSE requests
	Path string `toml:"path"`
	// CommentPings enables sending pings as SSE comments (": ping") instead of ping events.
	// Comments are ignored by EventSource, so clients can't rely on pings to detect stale connections.
	CommentPings bool `toml:"comment_pings"`
	// Zero means using the global ping interval (if it's been changed) or DefaultPingInterval.
	PingInterval   int    `toml:"ping_interval"`
	AllowedOrigins string `toml:"-"`
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

	result.WriteString("# Send pings as SSE comments (\": ping\") instead of ping events (always enabled in raw mode)\n")
	if c.CommentPings {
		result.WriteString("comment_pings = true\n")
	} else {
		result.WriteString("# comment_pings = true\n")
	}

	result.WriteString("# Keepalive interval for comment-based pings (seconds)\n")
	result.WriteString("# Defaults to the global ping_interval (if changed) or " + fmt.Sprintf("%d", DefaultPingInterval) + "\n")
	if c.PingInterval > 0 {
		result.WriteString(fmt.Sprintf("ping_interval = %d\n", c.PingInterval))
	} else {
		result.WriteString(fmt.Sprintf("# ping_interval = %d\n", DefaultPingInterval))
	}

	result.WriteString("\n")

	return result.String()
}
