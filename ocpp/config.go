package ocpp

import (
	"fmt"
	"strings"
)

type Config struct {
	// ChannelName is the name of the channel to use at the Action Cable side
	ChannelName string
	// WebSocket endpoint path to accept OCPP connections.
	// It will be suffixed with /:station_id to distinguish between stations.
	Path        string
	IdleTimeout int
	// Default heartbeat interval in seconds
	HeartbeatInterval int
	// GranularActions defines how to invoke Action Cable via RPC: by calling different methods for
	// different commands or by calling a single #receive method for all commands.
	GranularActions bool
}

func NewConfig() Config {
	return Config{
		ChannelName:       "OCPPChannel",
		Path:              "",
		IdleTimeout:       10,
		HeartbeatInterval: 30,
		GranularActions:   true,
	}
}

func (c Config) Enabled() bool {
	return c.Path != ""
}

func (c Config) ToToml() string {
	var result strings.Builder

	result.WriteString("# OCPP WebSocket endpoint path\n")
	result.WriteString(fmt.Sprintf("path = \"%s\"\n", c.Path))

	result.WriteString("# Action Cable channel class name\n")
	result.WriteString(fmt.Sprintf("action_cable_channel = \"%s\"\n", c.ChannelName))

	result.WriteString("# For how long to wait for connection_init msg (in seconds)\n")
	result.WriteString(fmt.Sprintf("idle_timeout = %d\n", c.IdleTimeout))

	result.WriteString("# Default heartbeat interval (seconds)\n")
	result.WriteString(fmt.Sprintf("heartbeat_interval = %d\n", c.HeartbeatInterval))

	result.WriteString("# Enable granular RPC actions for Action Cable via RPC\n")
	result.WriteString(fmt.Sprintf("granular_actions = %t\n", c.GranularActions))

	return result.String()
}
