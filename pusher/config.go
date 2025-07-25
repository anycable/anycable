package pusher

import (
	"fmt"
	"strings"
)

type Config struct {
	AppKey  string `toml:"app_key"`
	AuthKey string `toml:"auth_key"`
	Secret  string `toml:"secret"`
	// AddCORSHeaders enables adding CORS headers (so you can perform broadcast requests from the browser)
	// (We mostly need it for Stackblitz)
	AddCORSHeaders bool
	// CORSHosts contains a list of hostnames for CORS (comma-separated)
	CORSHosts string
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

	result.WriteString("# The secret key for Pusher clients\n")
	if c.Secret != "" {
		result.WriteString(fmt.Sprintf("secret = \"%s\"\n", c.Secret))
	} else {
		result.WriteString("# secret = \"\"\n")
	}

	result.WriteString("# The auth key for broadcasting Pusher events\n")
	if c.AuthKey != "" {
		result.WriteString(fmt.Sprintf("auth_key = \"%s\"\n", c.AuthKey))
	} else {
		result.WriteString("# auth_key = \"\"\n")
	}

	result.WriteString("\n")

	return result.String()
}
