package pusher

import (
	"fmt"
	"strings"
)

type Config struct {
	AppKey string `toml:"app_key"`
}

// NewConfig returns a new Config
func NewConfig() Config {
	return Config{}
}

func (c *Config) Enabled() bool {
	return c.AppKey != ""
}

func (c Config) ToToml() string {
	var result strings.Builder

	result.WriteString("# The public app key for Pusher clients\n")
	if c.AppKey != "" {
		result.WriteString(fmt.Sprintf("app_key = \"%s\"\n", c.AppKey))
	} else {
		result.WriteString("# app_key = \"\"\n")
	}

	result.WriteString("\n")

	return result.String()
}
