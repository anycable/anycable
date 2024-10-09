package nats

import (
	"fmt"
	"strings"

	natsgo "github.com/nats-io/nats.go"
)

type NATSConfig struct {
	Servers              string `toml:"servers"`
	DontRandomizeServers bool   `toml:"dont_randomize_servers"`
	MaxReconnectAttempts int    `toml:"max_reconnect_attempts"`
}

func NewNATSConfig() NATSConfig {
	return NATSConfig{
		Servers:              natsgo.DefaultURL,
		MaxReconnectAttempts: 5,
	}
}

func (c NATSConfig) ToToml() string {
	var result strings.Builder

	result.WriteString("# NATS server URLs (comma-separated)\n")
	result.WriteString(fmt.Sprintf("servers = \"%s\"\n", c.Servers))

	result.WriteString("# Don't randomize servers during connection\n")
	if c.DontRandomizeServers {
		result.WriteString("dont_randomize_servers = true\n")
	} else {
		result.WriteString("# dont_randomize_servers = true\n")
	}

	result.WriteString("# Max number of reconnect attempts\n")
	result.WriteString(fmt.Sprintf("max_reconnect_attempts = %d\n", c.MaxReconnectAttempts))

	result.WriteString("\n")

	return result.String()
}
