package node

import (
	"fmt"
	"strings"
)

const (
	DISCONNECT_MODE_ALWAYS = "always"
	DISCONNECT_MODE_AUTO   = "auto"
	DISCONNECT_MODE_NEVER  = "never"
)

var DISCONNECT_MODES = []string{DISCONNECT_MODE_ALWAYS, DISCONNECT_MODE_AUTO, DISCONNECT_MODE_NEVER}

// Config contains general application/node settings
type Config struct {
	// Define when to invoke Disconnect callback
	DisconnectMode string `toml:"disconnect_mode"`
	// The number of goroutines to use for disconnect calls on shutdown
	ShutdownDisconnectPoolSize int `toml:"shutdown_disconnect_gopool_size"`
	// How often server should send Action Cable ping messages (seconds)
	PingInterval int `toml:"ping_interval"`
	// How ofter to refresh node stats (seconds)
	StatsRefreshInterval int `toml:"stats_refresh_interval"`
	// The max size of the Go routines pool for hub
	HubGopoolSize int `toml:"broadcast_gopool_size"`
	// How should ping message timestamp be formatted? ('s' => seconds, 'ms' => milli seconds, 'ns' => nano seconds)
	PingTimestampPrecision string `toml:"ping_timestamp_precision"`
	// For how long to wait for pong message before disconnecting (seconds)
	PongTimeout int `toml:"pong_timeout"`
	// For how long to wait for disconnect callbacks to be processed before exiting (seconds)
	ShutdownTimeout int `toml:"shutdown_timeout"`
}

// NewConfig builds a new config
func NewConfig() Config {
	return Config{
		PingInterval:               3,
		StatsRefreshInterval:       5,
		HubGopoolSize:              16,
		ShutdownDisconnectPoolSize: 16,
		PingTimestampPrecision:     "s",
		DisconnectMode:             DISCONNECT_MODE_AUTO,
		ShutdownTimeout:            30,
	}
}

func (c Config) ToToml() string {
	var result strings.Builder

	result.WriteString("# Server-to-client heartbeat interval (seconds)\n")
	result.WriteString(fmt.Sprintf("ping_interval = %d\n", c.PingInterval))

	result.WriteString("# Timestamp format for ping messages (s, ms, or ns)\n")
	result.WriteString(fmt.Sprintf("ping_timestamp_precision = \"%s\"\n", c.PingTimestampPrecision))

	result.WriteString("# Client-to-server pong timeout (seconds)\n")
	if c.PongTimeout == 0 {
		result.WriteString("# pong_timeout = 6\n")
	} else {
		result.WriteString(fmt.Sprintf("pong_timeout = %d\n", c.PongTimeout))
	}

	result.WriteString("# Define when to invoke Disconnect RPC callback\n")
	result.WriteString(fmt.Sprintf("disconnect_mode = \"%s\"\n", c.DisconnectMode))

	result.WriteString("# Graceful shutdown period (seconds)\n")
	result.WriteString(fmt.Sprintf("shutdown_timeout = %d\n", c.ShutdownTimeout))

	result.WriteString("# How often to refresh system-wide metrics (seconds)\n")
	result.WriteString(fmt.Sprintf("stats_refresh_interval = %d\n", c.StatsRefreshInterval))

	result.WriteString("# The number of Go routines to use for broadcasting (server-to-client fan-out)\n")
	result.WriteString(fmt.Sprintf("broadcast_gopool_size = %d\n", c.HubGopoolSize))

	result.WriteString("# The number of Go routines to use for Disconnect RPC calls on shutdown\n")
	result.WriteString(fmt.Sprintf("shutdown_disconnect_gopool_size = %d\n", c.ShutdownDisconnectPoolSize))

	result.WriteString("\n")

	return result.String()
}
