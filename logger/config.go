package logger

import (
	"fmt"
	"strings"
)

type Config struct {
	LogLevel  string `toml:"level"`
	LogFormat string `toml:"format"`
	Debug     bool   `toml:"debug"`
}

func NewConfig() Config {
	return Config{
		LogLevel:  "info",
		LogFormat: "text",
	}
}

func (c Config) ToToml() string {
	var result strings.Builder

	result.WriteString("# Logging level (debug, info, warn, error)\n")
	result.WriteString(fmt.Sprintf("level = \"%s\"\n", c.LogLevel))

	result.WriteString("# Logs formatting (e.g., 'text' or 'json')\n")
	result.WriteString(fmt.Sprintf("format = \"%s\"\n", c.LogFormat))

	result.WriteString("# Enable debug (verbose) logging\n")
	if c.Debug {
		result.WriteString("debug = true\n")
	} else {
		result.WriteString("# debug = true\n")
	}

	result.WriteString("\n")

	return result.String()
}
