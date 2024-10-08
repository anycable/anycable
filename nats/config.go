package nats

import (
	"fmt"
	"strings"

	natsgo "github.com/nats-io/nats.go"
)

type NATSConfig struct {
	Servers              string `toml:"servers"`
	Channel              string `toml:"channel"`
	DontRandomizeServers bool   `toml:"dont_randomize_servers"`
	MaxReconnectAttempts int    `toml:"max_reconnect_attempts"`
	// Internal channel name for node-to-node broadcasting
	InternalChannel string `toml:"internal_channel"`
}

func NewNATSConfig() NATSConfig {
	return NATSConfig{
		Servers:              natsgo.DefaultURL,
		Channel:              "__anycable__",
		MaxReconnectAttempts: 5,
		InternalChannel:      "__anycable_internal__",
	}
}

func (c NATSConfig) ToToml() string {
	var result strings.Builder

	result.WriteString("# NATS server URLs (comma-separated)\n")
	result.WriteString(fmt.Sprintf("servers = \"%s\"\n", c.Servers))

	result.WriteString("# Channel name for legacy broadasting\n")
	result.WriteString(fmt.Sprintf("channel = \"%s\"\n", c.Channel))

	result.WriteString("# Don't randomize servers during connection\n")
	if c.DontRandomizeServers {
		result.WriteString("dont_randomize_servers = true\n")
	} else {
		result.WriteString("# dont_randomize_servers = true\n")
	}

	result.WriteString("# Max number of reconnect attempts\n")
	result.WriteString(fmt.Sprintf("max_reconnect_attempts = %d\n", c.MaxReconnectAttempts))

	result.WriteString("# Channel name for pub/sub (node-to-node)\n")
	result.WriteString(fmt.Sprintf("internal_channel = \"%s\"\n", c.InternalChannel))

	result.WriteString("\n")

	return result.String()
}
