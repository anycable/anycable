package rpc

// Config contains RPC controller configuration
type Config struct {
	Host string
}

// NewConfig builds a new config
func NewConfig() Config {
	return Config{}
}
