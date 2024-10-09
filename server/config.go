package server

import (
	"fmt"
	"strings"
)

type Config struct {
	Host           string    `toml:"host"`
	Port           int       `toml:"port"`
	AllowedOrigins string    `toml:"allowed_origins"`
	MaxConn        int       `toml:"max_conn"`
	HealthPath     string    `toml:"health_path"`
	SSL            SSLConfig `toml:"ssl"`
}

func NewConfig() Config {
	return Config{
		Host:       "localhost",
		Port:       8080,
		HealthPath: "/health",
		SSL:        NewSSLConfig(),
	}
}

func (c Config) ToToml() string {
	var result strings.Builder

	result.WriteString("# Host address to bind to\n")
	result.WriteString(fmt.Sprintf("host = %q\n", c.Host))
	result.WriteString("# Port to listen on\n")
	result.WriteString(fmt.Sprintf("port = %d\n", c.Port))

	result.WriteString("# Allowed origins (a comma-separated list)\n")
	result.WriteString(fmt.Sprintf("allowed_origins = \"%s\"\n", c.AllowedOrigins))

	result.WriteString("# Maximum number of allowed concurrent connections\n")
	if c.MaxConn == 0 {
		result.WriteString("# max_conn = 1000\n")
	} else {
		result.WriteString(fmt.Sprintf("max_conn = %d\n", c.MaxConn))
	}
	result.WriteString("# Health check endpoint path\n")
	result.WriteString(fmt.Sprintf("health_path = %q\n", c.HealthPath))

	result.WriteString("# SSL configuration\n")

	if c.SSL.CertPath != "" {
		result.WriteString(fmt.Sprintf("ssl.cert_path = %q\n", c.SSL.CertPath))
	} else {
		result.WriteString("# ssl.cert_path =\n")
	}

	if c.SSL.KeyPath != "" {
		result.WriteString(fmt.Sprintf("ssl.key_path = %q\n", c.SSL.KeyPath))
	} else {
		result.WriteString("# ssl.key_path =\n")
	}

	result.WriteString("\n")

	return result.String()
}
