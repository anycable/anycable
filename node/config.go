package node

const (
	DISCONNECT_MODE_ALWAYS = "always"
	DISCONNECT_MODE_AUTO   = "auto"
	DISCONNECT_MODE_NEVER  = "never"
)

var DISCONNECT_MODES = []string{DISCONNECT_MODE_ALWAYS, DISCONNECT_MODE_AUTO, DISCONNECT_MODE_NEVER}

// Config contains general application/node settings
type Config struct {
	// Define when to invoke Disconnect callback
	DisconnectMode string
	// The number of goroutines to use for disconnect calls on shutdown
	ShutdownDisconnectPoolSize int
	// How often server should send Action Cable ping messages (seconds)
	PingInterval int
	// How ofter to refresh node stats (seconds)
	StatsRefreshInterval int
	// The max size of the Go routines pool for hub
	HubGopoolSize int
	// Whether to use net polling for reading data or spawn a go routine
	NetpollEnabled bool
	// The max size of the Go routines pool to process inbound client messages
	ReadGopoolSize int
	// The max size of the Go routines pool to process outbound client messages
	WriteGopoolSize int
	// How should ping message timestamp be formatted? ('s' => seconds, 'ms' => milli seconds, 'ns' => nano seconds)
	PingTimestampPrecision string
	// For how long to wait for pong message before disconnecting (seconds)
	PongTimeout int
	// For how long to wait for disconnect callbacks to be processed before exiting (seconds)
	ShutdownTimeout int
	// Enable slow draining of the connections during shutdown
	SlowDrainShutdown bool
	// The max interval between slow drain disconnect calls (milliseconds)
	SlowDrainMaxInterval int
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
		PingTimestampPrecision:     "s",
		DisconnectMode:             DISCONNECT_MODE_AUTO,
		ShutdownTimeout:            30,
		SlowDrainMaxInterval:       500,
	}
}
