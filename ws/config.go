package ws

import (
	"fmt"
	"strings"
)

// Config contains WebSocket connection configuration.
type Config struct {
	Paths             []string `toml:"paths"`
	ReadBufferSize    int      `toml:"read_buffer_size"`
	WriteBufferSize   int      `toml:"write_buffer_size"`
	MaxMessageSize    int64    `toml:"max_message_size"`
	WriteTimeout      int      `toml:"write_timeout"`
	MaxPendingSize    uint64   `toml:"max_pending_size"`
	EnableCompression bool     `toml:"enable_compression"`
	AllowedOrigins    string   `toml:"-"`
}

// NewConfig build a new Config struct
func NewConfig() Config {
	return Config{
		Paths:           []string{"/cable"},
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		MaxMessageSize:  65536,
		MaxPendingSize:  1024 * 1024, // 1 MB
		WriteTimeout:    2,
	}
}

// ToToml converts the Config struct to a TOML string representation
func (c Config) ToToml() string {
	var result strings.Builder

	result.WriteString("# WebSocket endpoint paths\n")
	result.WriteString(fmt.Sprintf("paths = [\"%s\"]\n", strings.Join(c.Paths, "\", \"")))

	result.WriteString("# Read buffer size\n")
	result.WriteString(fmt.Sprintf("read_buffer_size = %d\n", c.ReadBufferSize))

	result.WriteString("# Write buffer size\n")
	result.WriteString(fmt.Sprintf("write_buffer_size = %d\n", c.WriteBufferSize))

	result.WriteString("# Maximum message size\n")
	result.WriteString(fmt.Sprintf("max_message_size = %d\n", c.MaxMessageSize))

	result.WriteString("# Write timeout (seconds)\n")
	result.WriteString(fmt.Sprintf("write_timeout = %d\n", c.WriteTimeout))

	if c.MaxPendingSize > 0 {
		result.WriteString("# Maximum pending size\n")
		result.WriteString(fmt.Sprintf("max_pending_size = %d\n", c.MaxPendingSize))
	} else {
		result.WriteString("# Maximum pending size\n")
		result.WriteString("# max_pending_size = 1048576\n")
	}

	if c.EnableCompression {
		result.WriteString("# Enable compression (per-message deflate)\n")
		result.WriteString("enable_compression = true\n")
		result.WriteString("# enable_compression = true\n")
	}

	result.WriteString("\n")

	return result.String()
}
