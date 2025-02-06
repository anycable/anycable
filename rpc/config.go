package rpc

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	pb "github.com/anycable/anycable-go/protos"
)

const (
	defaultRPCHost = "localhost:50051"
	// Slightly less than default Ruby gRPC server concurrency
	defaultRPCConcurrency = 28
)

// ClientHelper provides additional methods to operate gRPC client
type ClientHelper interface {
	Ready() error
	SupportsActiveConns() bool
	ActiveConns() int
	Close()
}

// Dialer is factory function to build a new client with its helper
type Dialer = func(c *Config, l *slog.Logger) (pb.RPCClient, ClientHelper, error)

// Config contains RPC controller configuration
type Config struct {
	// RPC instance host
	Host string `toml:"host"`
	// ProxyHeaders to add to RPC request env
	ProxyHeaders []string `toml:"proxy_headers"`
	// ProxyCookies to add to RPC request env
	ProxyCookies []string `toml:"proxy_cookies"`
	// The max number of simultaneous requests.
	// Should be slightly less than the RPC server concurrency to avoid
	// ResourceExhausted errors
	Concurrency int `toml:"concurrency"`
	// Enable client-side TLS on RPC connections?
	EnableTLS bool `toml:"enable_tls"`
	// Whether to verify the RPC server's certificate chain and host name
	TLSVerify bool `toml:"tls_verify"`
	// CA root TLS certificate path
	TLSRootCA string `toml:"tls_root_ca_path"`
	// Max receive msg size (bytes)
	MaxRecvSize int `toml:"max_recv_size"`
	// Max send msg size (bytes)
	MaxSendSize int `toml:"max_send_size"`
	// Underlying implementation (grpc, http, or none)
	Implementation string `toml:"implementation"`
	// Alternative dialer implementation
	DialFun Dialer
	// Secret for HTTP RPC authentication
	Secret string `toml:"secret"`
	// Request timeout (in ms) (for all RPC implementations)
	RequestTimeout int `toml:"request_timeout"`
	// Command execution timeout (in ms) (may consist of multiple request retries)
	CommandTimeout int `toml:"command_timeout"`
	// Timeout for HTTP RPC requests (in ms)
	HTTPRequestTimeout int `toml:"http_request_timeout"`
	// SecretBase is a secret used to generate authentication token
	SecretBase string
}

// NewConfig builds a new config
func NewConfig() Config {
	return Config{
		ProxyHeaders:   []string{"cookie"},
		Concurrency:    defaultRPCConcurrency,
		EnableTLS:      false,
		TLSVerify:      true,
		Host:           defaultRPCHost,
		Implementation: "",
		// TODO(v2): Change the default in v2 to some 5s or whatever
		RequestTimeout:     0,
		HTTPRequestTimeout: 0,
		CommandTimeout:     3000,
	}
}

// Return chosen implementation either from the user provided value
// or from the host scheme
func (c *Config) Impl() string {
	if c.Implementation != "" {
		return c.Implementation
	}

	uri, err := url.Parse(ensureGrpcScheme(c.Host))

	if err != nil {
		return fmt.Sprintf("<invalid RPC host: %s>", c.Host)
	}

	if uri.Scheme == "http" || uri.Scheme == "https" {
		return "http"
	}

	return "grpc"
}

// Whether secure connection to RPC server is enabled either explicitly or implicitly
func (c *Config) TLSEnabled() bool {
	return c.EnableTLS || c.TLSRootCA != ""
}

// TLSConfig builds TLS configuration for RPC client
func (c *Config) TLSConfig() (*tls.Config, error) {
	if !c.TLSEnabled() {
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

// Backward-compatibility to provide a default value
func (c *Config) GetHTTPRequestTimeout() int {
	if c.HTTPRequestTimeout > 0 {
		return c.HTTPRequestTimeout
	}

	if c.RequestTimeout > 0 {
		return c.RequestTimeout
	}

	// legacy default value
	return 3000
}

func ensureGrpcScheme(url string) string {
	if strings.Contains(url, "://") {
		return url
	}

	return "grpc://" + url
}

func (c Config) ToToml() string {
	var result strings.Builder

	result.WriteString("# RPC implementation (grpc, http, or none)\n")
	result.WriteString(fmt.Sprintf("implementation = \"%s\"\n", c.Implementation))

	result.WriteString("# RPC service hostname (including port, e.g., 'anycable-rpc:50051')\n")
	result.WriteString(fmt.Sprintf("host = \"%s\"\n", c.Host))

	result.WriteString("# Specify HTTP headers that must be proxied to the RPC service\n")
	if len(c.ProxyHeaders) > 0 {
		result.WriteString(fmt.Sprintf("proxy_headers = [\"%s\"]\n", strings.Join(c.ProxyHeaders, "\", \"")))
	} else {
		result.WriteString("# proxy_headers = [\"cookie\"]\n")
	}

	result.WriteString("# Specify which cookies must be kept in the proxied Cookie header\n")
	if len(c.ProxyCookies) > 0 {
		result.WriteString(fmt.Sprintf("proxy_cookies = [\"%s\"]\n", strings.Join(c.ProxyCookies, "\", \"")))
	} else {
		result.WriteString("# proxy_cookies = [\"_session_id\"]\n")
	}

	result.WriteString("# RPC concurrency (max number of concurrent RPC requests)\n")
	result.WriteString(fmt.Sprintf("concurrency = %d\n", c.Concurrency))

	result.WriteString("# Enable client-side TLS on RPC connections\n")
	if c.EnableTLS {
		result.WriteString(fmt.Sprintf("enable_tls = %v\n", c.EnableTLS))
	} else {
		result.WriteString("# enable_tls = true\n")
	}

	result.WriteString("# Enable TLS Verify for RPC connections\n")
	if c.TLSVerify {
		result.WriteString(fmt.Sprintf("tls_verify = %v\n", c.TLSVerify))
	} else {
		result.WriteString("# tls_verify = true\n")
	}

	result.WriteString("# CA root TLS certificate path\n")
	if c.TLSRootCA == "" {
		result.WriteString(fmt.Sprintf("tls_root_ca_path = \"%s\"\n", c.TLSRootCA))
	} else {
		result.WriteString("# tls_root_ca_path =\n")
	}

	result.WriteString("# HTTP RPC specific settings\n")
	result.WriteString("# Secret for HTTP RPC authentication\n")
	if c.Secret != "" {
		result.WriteString(fmt.Sprintf("secret = \"%s\"\n", c.Secret))
	} else {
		result.WriteString("# secret =\n")
	}

	result.WriteString("# Timeout for RPC requests (in ms)\n")
	if c.RequestTimeout > 0 {
		result.WriteString(fmt.Sprintf("request_timeout = %d\n", c.RequestTimeout))
	} else {
		result.WriteString("# request_timeout = 3000\n")
	}

	result.WriteString("# Total timeout for RPC commands including retries (in ms)\n")
	if c.CommandTimeout > 0 {
		result.WriteString(fmt.Sprintf("command_timeout = %d\n", c.CommandTimeout))
	} else {
		result.WriteString("# command_timeout = 3000\n")
	}

	result.WriteString("# GRPC fine-tuning\n")
	result.WriteString("# Max allowed incoming message size (bytes)\n")
	result.WriteString(fmt.Sprintf("max_recv_size = %d\n", c.MaxRecvSize))

	result.WriteString("# Max allowed outgoing message size (bytes)\n")
	result.WriteString(fmt.Sprintf("max_send_size = %d\n", c.MaxSendSize))

	result.WriteString("\n")

	return result.String()
}
