package rpc

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	pb "github.com/anycable/anycable-go/protos"
)

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
	// Whether to verify the RPC server's certificate chain and host name
	TLSVerify bool
	// CA root TLS certificate path
	TLSRootCA string
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
	return Config{Concurrency: 28, EnableTLS: false, TLSVerify: true, Host: defaultRPCHost, Implementation: "grpc", RequestTimeout: 3000}
}

func (c *Config) TLSConfig() (*tls.Config, error) {
	if !c.EnableTLS {
		return nil, nil
	}

	var certPool *x509.CertPool = nil // use system CA certificates
	if c.TLSRootCA != "" {
		var rootCertificate []byte
		var error error
		if info, err := os.Stat(c.TLSRootCA); !os.IsNotExist(err) && !info.IsDir() {
			rootCertificate, error = os.ReadFile(c.TLSRootCA)
			if error != nil {
				return nil, fmt.Errorf("failed to read RPC root CA certificate: %s", error)
			}
		} else {
			rootCertificate = []byte(c.TLSRootCA)
		}

		certPool = x509.NewCertPool()
		ok := certPool.AppendCertsFromPEM(rootCertificate)
		if !ok {
			return nil, errors.New("failed to parse RPC root CA certificate")
		}
	}

	// #nosec G402: InsecureSkipVerify explicitly allowed to be set to true for development/testing
	tlsConfig := &tls.Config{
		InsecureSkipVerify: !c.TLSVerify,
		MinVersion:         tls.VersionTLS12,
		RootCAs:            certPool,
	}

	return tlsConfig, nil
}
