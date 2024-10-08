package metrics

import (
	"fmt"
	"strings"
)

// Config contains metrics configuration
type Config struct {
	Log            bool `toml:"log"`
	LogInterval    int  // Deprecated
	RotateInterval int  `toml:"rotate_interval"`
	LogFormatter   string
	// Print only specified metrics
	LogFilter []string          `toml:"log_filter"`
	HTTP      string            `toml:"http_path"`
	Host      string            `toml:"host"`
	Port      int               `toml:"port"`
	Tags      map[string]string `toml:"tags"`
	Statsd    StatsdConfig      `toml:"statsd"`
}

// NewConfig creates an empty Config struct
func NewConfig() Config {
	return Config{
		RotateInterval: 15,
		Statsd:         NewStatsdConfig(),
	}
}

// LogEnabled returns true iff any log option is specified
func (c *Config) LogEnabled() bool {
	return c.Log || c.LogFormatterEnabled()
}

// HTTPEnabled returns true iff HTTP is not empty
func (c *Config) HTTPEnabled() bool {
	return c.HTTP != ""
}

// LogFormatterEnabled returns true iff LogFormatter is not empty
func (c *Config) LogFormatterEnabled() bool {
	return c.LogFormatter != ""
}

// ToToml converts the Config to a TOML string representation
func (c Config) ToToml() string {
	var result strings.Builder

	result.WriteString("# HTTP endpoint (Prometheus)\n")
	if c.HTTP != "" {
		result.WriteString(fmt.Sprintf("http = \"%s\"\n", c.HTTP))
	} else {
		result.WriteString("# http = \"/metrics\"\n")
	}

	result.WriteString("# Standalone metrics HTTP server host to bind to\n")
	if c.Host != "" {
		result.WriteString(fmt.Sprintf("host = \"%s\"\n", c.Host))
	} else {
		result.WriteString("# host = \"localhost\"\n")
	}

	result.WriteString("# Metrics HTTP server port to listen on\n# (can be the same as the main server's port)\n")
	if c.Port != 0 {
		result.WriteString(fmt.Sprintf("port = %d\n", c.Port))
	} else {
		result.WriteString("# port = 8082\n")
	}

	result.WriteString("# Enable metrics logging\n")
	if c.Log {
		result.WriteString("log = true\n")
	} else {
		result.WriteString("# log = true\n")
	}

	result.WriteString("# Log rotation interval (seconds)\n")
	result.WriteString(fmt.Sprintf("rotate_interval = %d\n", c.RotateInterval))

	result.WriteString("# Log filter (show only selected metrics)\n")
	if len(c.LogFilter) > 0 {
		result.WriteString(fmt.Sprintf("log_filter = [ \"%s\" ]\n", strings.Join(c.LogFilter, "\", \"")))
	} else {
		result.WriteString("# log_filter = []\n")
	}

	result.WriteString("# Metrics tags\n")
	if len(c.Tags) > 0 {
		for key, value := range c.Tags {
			result.WriteString(fmt.Sprintf("tags.%s = \"%s\"\n", key, value))
		}
	} else {
		result.WriteString("# tags.key = \"value\"\n")
	}

	result.WriteString("\n")

	return result.String()
}
