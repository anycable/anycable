package node

// Config contains general application/node settings
type Config struct {
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
}

// NewConfig builds a new config
func NewConfig() Config {
	return Config{PingInterval: 3, StatsRefreshInterval: 5, NetpollEnabled: true, HubGopoolSize: 16, WriteGopoolSize: 1024, ReadGopoolSize: 1024, PingTimestampPrecision: "s"}
}
