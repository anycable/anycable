package api

import (
	"fmt"
	"strings"

	"github.com/anycable/anycable-go/utils"
)

const (
	defaultAPIPath = "/api"
	apiKeyPhrase   = "api-cable"
)

// Config contains API server configuration
type Config struct {
	// Host to bind the API server (only used when Port is different from main server)
	Host string `toml:"host"`
	// Port to listen on (0 = use main server port)
	Port int `toml:"port"`
	// Path is the base path for API endpoints
	Path string `toml:"path"`
	// Secret token to authorize requests
	Secret string `toml:"secret"`
	// SecretBase is a secret used to generate a token if none provided
	SecretBase string
	// AddCORSHeaders enables adding CORS headers
	AddCORSHeaders bool `toml:"cors_headers"`
	// CORSHosts contains a list of hostnames for CORS (comma-separated)
	CORSHosts string `toml:"cors_hosts"`
}

// NewConfig returns a new Config with default values
func NewConfig() Config {
	return Config{
		Path: defaultAPIPath,
	}
}

// IsSecured returns true if authentication is configured
func (c *Config) IsSecured() bool {
	return c.Secret != "" || c.SecretBase != ""
}

// DeriveSecret generates the secret from SecretBase if Secret is not set
func (c *Config) DeriveSecret() error {
	if c.Secret != "" {
		return nil
	}

	if c.SecretBase == "" {
		return nil
	}

	secret, err := utils.NewMessageVerifier(c.SecretBase).Sign([]byte(apiKeyPhrase))
	if err != nil {
		return fmt.Errorf("failed to auto-generate authentication key for API: %w", err)
	}

	c.Secret = string(secret)
	return nil
}

func (c Config) ToToml() string {
	var result strings.Builder

	result.WriteString("# API server host (only used when port differs from main server)\n")
	if c.Host != "" {
		result.WriteString(fmt.Sprintf("host = \"%s\"\n", c.Host))
	} else {
		result.WriteString("# host = \"localhost\"\n")
	}

	result.WriteString("# API server port (0 = use main server port)\n")
	result.WriteString(fmt.Sprintf("port = %d\n", c.Port))

	result.WriteString("# Base path for API endpoints\n")
	result.WriteString(fmt.Sprintf("path = \"%s\"\n", c.Path))

	result.WriteString("# Secret token to authenticate API requests\n")
	if c.Secret != "" {
		result.WriteString(fmt.Sprintf("secret = \"%s\"\n", c.Secret))
	} else {
		result.WriteString("# secret = \"\"\n")
	}

	result.WriteString("# Enable CORS headers\n")
	if c.AddCORSHeaders {
		result.WriteString("cors_headers = true\n")
	} else {
		result.WriteString("# cors_headers = false\n")
	}

	result.WriteString("\n")

	return result.String()
}
