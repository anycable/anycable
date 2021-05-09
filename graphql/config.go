package graphql

import (
	"fmt"
	"strings"
)

type Config struct {
	// Where to mount WS handler
	Path string `toml:"path"`
	// Action Cable channel class name
	Channel string `toml:"action_cable_channel"`
	// Action Cable channel action name
	Action string `toml:"channel_action"`
}

const (
	// From https://github.com/apollographql/subscriptions-transport-ws/blob/master/src/protocol.ts
	LegacyGraphQLProtocol = "graphql-ws"
)

func GraphqlProtocols() []string {
	return []string{LegacyGraphQLProtocol}
}

func NewConfig() Config {
	return Config{Channel: "GraphqlChannel", Action: "execute"}
}

func (c Config) Enabled() bool {
	return c.Path != ""
}

func (c Config) ToToml() string {
	var result strings.Builder

	result.WriteString("# GraphQL WebSocket endpoint path\n")
	result.WriteString(fmt.Sprintf("path = \"%s\"\n", c.Path))

	result.WriteString("# Action Cable channel class name\n")
	result.WriteString(fmt.Sprintf("action_cable_channel = \"%s\"\n", c.Channel))

	result.WriteString("# Action Cable channel action name\n")
	result.WriteString(fmt.Sprintf("channel_action = \"%s\"\n", c.Action))

	result.WriteString("\n")

	return result.String()
}
