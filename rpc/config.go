package rpc

import pb "github.com/anycable/anycable-go/protos"

const (
	defaultRPCHost = "localhost:50051"
)

// ClientHelper provides additional methods to operate gRPC client
type ClientHelper interface {
	Ready() error
	SupportsActiveConns() bool
	ActiveConns() int
	Close()
}

// Dialer is factory function to build a new client with its helper
type Dialer = func(c *Config) (pb.RPCClient, ClientHelper, error)

// Config contains RPC controller configuration
type Config struct {
	// RPC instance host
	Host string
	// The max number of simultaneous requests.
	// Should be slightly less than the RPC server concurrency to avoid
	// ResourceExhausted errors
	Concurrency int
	// Enable client-side TLS on RPC connections?
	EnableTLS bool
	// Max receive msg size (bytes)
	MaxRecvSize int
	// Max send msg size (bytes)
	MaxSendSize int
	// Underlying implementation (grpc, http)
	Implementation string
	// Alternative dialer implementation
	DialFun Dialer
	// Secret for HTTP RPC authentication
	Secret string
	// Timeout for HTTP RPC requests (in ms)
	RequestTimeout int
}

// NewConfig builds a new config
func NewConfig() Config {
	return Config{Concurrency: 28, EnableTLS: false, Host: defaultRPCHost, Implementation: "grpc", RequestTimeout: 3000}
}
