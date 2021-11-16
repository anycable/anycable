package rpc

// Config contains RPC controller configuration
type Config struct {
	// RPC instance host
	Host string
	// The max number of simulteneous requests.
	// Should be slightly less than the RPC server concurrency to avoid
	// ResourceExhausted errors
	Concurrency int
	// Enable client-side TLS on RPC connections?
	EnableTLS bool
	// Max recieve msg size (bytes)
	MaxRecvSize int
	// Max send msg size (bytes)
	MaxSendSize int
}

// NewConfig builds a new config
func NewConfig() Config {
	return Config{Concurrency: 28, EnableTLS: false}
}
