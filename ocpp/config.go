package ocpp

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
