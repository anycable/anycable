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
	// Whether to use net polling for reading data or spawn a go routine
	NetpollEnabled bool
	// The max size of the Go routines pool to process inbound client messages
	ReadGopoolSize int
	// For how long to wait before the processing of the incoming message starts by the read pool (in ms)
	ReadPoolTimeout int `toml:"read_queue_timeout"`
	// The max size of the Go routines pool to process outbound client messages
	WriteGopoolSize int
	// How should ping message timestamp be formatted? ('s' => seconds, 'ms' => milli seconds, 'ns' => nano seconds)
	PingTimestampPrecision string `toml:"ping_timestamp_precision"`
	// For how long to wait for pong message before disconnecting (seconds)
	PongTimeout int `toml:"pong_timeout"`
	// Send native pings (e.g., WebSocket ping frames) along with application-level pings to keepalive clients using custom protocols
	EnableNativePing bool `toml:"enable_native_ping"`
	// For how long to wait for disconnect callbacks to be processed before exiting (seconds)
	ShutdownTimeout int `toml:"shutdown_timeout"`
	// For how long to sleep before shutting down
	ShutdownDelay int `toml:"shutdown_delay"`
	// Enable slow draining of the connections during shutdown
	SlowDrainShutdown bool `toml:"slow_drain_shutdown"`
	// The max interval between slow drain disconnect calls (milliseconds)
	SlowDrainMaxInterval int `toml:"slow_drain_max_interval"`
}

// NewConfig builds a new config
func NewConfig() Config {
	return Config{
		PingInterval:               3,
		StatsRefreshInterval:       5,
		NetpollEnabled:             true,
		HubGopoolSize:              16,
		ShutdownDisconnectPoolSize: 16,
		WriteGopoolSize:            1024,
		ReadGopoolSize:             1024,
		ReadPoolTimeout:            3000,
		PingTimestampPrecision:     "s",
		DisconnectMode:             DISCONNECT_MODE_AUTO,
		ShutdownTimeout:            30,
		SlowDrainMaxInterval:       500,
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

	result.WriteString("# Enable native ping/pong mechanism (e.g., WebSocket PING/PONG frames)")
	if c.EnableNativePing {
		result.WriteString("# enable_native_ping = true\n")
	} else {
		result.WriteString("enable_native_ping = true\n")
	}

	result.WriteString("# Define when to invoke Disconnect RPC callback\n")
	result.WriteString(fmt.Sprintf("disconnect_mode = \"%s\"\n", c.DisconnectMode))

	result.WriteString("# Graceful shutdown period (seconds)\n")
	result.WriteString(fmt.Sprintf("shutdown_timeout = %d\n", c.ShutdownTimeout))

	result.WriteString("# Enable slow draining of the connections during shutdown\n")
	result.WriteString(fmt.Sprintf("slow_drain_shutdown = %t\n", c.SlowDrainShutdown))

	result.WriteString("# The max interval between slow drain disconnect calls (milliseconds)\n")
	result.WriteString(fmt.Sprintf("slow_drain_max_interval = %d\n", c.SlowDrainMaxInterval))

	result.WriteString("# How often to refresh system-wide metrics (seconds)\n")
	result.WriteString(fmt.Sprintf("stats_refresh_interval = %d\n", c.StatsRefreshInterval))

	result.WriteString("# The number of Go routines to use for broadcasting (server-to-client fan-out)\n")
	result.WriteString(fmt.Sprintf("broadcast_gopool_size = %d\n", c.HubGopoolSize))

	result.WriteString("# The number of Go routines to use for Disconnect RPC calls on shutdown\n")
	result.WriteString(fmt.Sprintf("shutdown_disconnect_gopool_size = %d\n", c.ShutdownDisconnectPoolSize))

	result.WriteString("# Use net polling read data from clients to reduce RAM usage (only affects WebSockets)\n")
	result.WriteString(fmt.Sprintf("netpoll_enabled = %t\n", c.NetpollEnabled))

	result.WriteString("# The number of Go routines to use for reading data from clients\n")
	result.WriteString(fmt.Sprintf("read_gopool_size = %d\n", c.ReadGopoolSize))

	result.WriteString("# The max delay before processing incoming messages in the read pool (milliseconds)\n")
	result.WriteString(fmt.Sprintf("read_queue_timeout = %d\n", c.ReadPoolTimeout))

	result.WriteString("# The number of Go routines to use for writing data to clients\n")
	result.WriteString(fmt.Sprintf("write_gopool_size = %d\n", c.WriteGopoolSize))

	result.WriteString("\n")

	return result.String()
}
