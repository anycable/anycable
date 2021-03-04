package node

// Config contains general application/node settings
type Config struct {
	// How often server should send Action Cable ping messages (seconds)
	PingInterval int
	// How ofter to refresh node stats (seconds)
	StatsRefreshInterval int
}

// NewConfig builds a new config
func NewConfig() Config {
	return Config{PingInterval: 3, StatsRefreshInterval: 5}
}
