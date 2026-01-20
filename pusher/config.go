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
	// Port for Pusher HTTP API (0 = use the main server port)
	APIPort int `toml:"api_port"`
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

	result.WriteString("# Port for Pusher HTTP API (0 = use the main server port)\n")
	if c.APIPort != 0 {
		result.WriteString(fmt.Sprintf("api_port = %d\n", c.APIPort))
	} else {
		result.WriteString("# api_port = 0\n")
	}

	result.WriteString("\n")

	return result.String()
}
