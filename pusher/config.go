package pusher

import (
	"fmt"
	"strings"
)

type Config struct {
	// Pusher application ID
	AppID string `toml:"app_id"`
	// Pusher application auth key
	AppKey string `toml:"app_key"`
	// Pusher secret
	Secret string `toml:"secret"`
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
	return c.AppID != "" && c.AppKey != ""
}

func (c Config) ToToml() string {
	var result strings.Builder

	result.WriteString("# Pusher application ID\n")
	if c.AppID != "" {
		result.WriteString(fmt.Sprintf("app_id = \"%s\"\n", c.AppID))
	} else {
		result.WriteString("# app_id = \"\"\n")
	}

	result.WriteString("# Pusher application authentication key\n")
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

	result.WriteString("\n")

	return result.String()
}
