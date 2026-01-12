package redis

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/rueidis"
)

// RedisConfig contains Redis pubsub adapter configuration
type RedisConfig struct {
	// Redis instance URL or master name in case of sentinels usage
	// or list of URLs if cluster usage
	URL string `toml:"url"`
	// List of Redis Sentinel addresses
	Sentinels string `toml:"sentinels"`
	// Redis Sentinel discovery interval (seconds)
	SentinelDiscoveryInterval int `toml:"sentinel_discovery_interval"`
	// Redis keepalive ping interval (seconds)
	KeepalivePingInterval int `toml:"keepalive_ping_interval"`
	// Whether to check server's certificate for validity (in case of rediss:// protocol)
	TLSVerify bool `toml:"tls_verify"`
	// Path to CA certificate file to verify Redis server certificate
	TLSCACertPath string `toml:"tls_ca_cert_path"`
	// Path to client TLS certificate file for mutual TLS
	TLSClientCertPath string `toml:"tls_client_cert_path"`
	// Path to client TLS private key file for mutual TLS
	TLSClientKeyPath string `toml:"tls_client_key_path"`
	// Max number of reconnect attempts
	MaxReconnectAttempts int `toml:"max_reconnect_attempts"`
	// Disable client-side caching
	DisableCache bool `toml:"disable_cache"`

	// List of hosts to connect
	hosts []string
	// Sentinel Master host to connect
	sentinelMaster string

	mu sync.RWMutex
}

// NewRedisConfig builds a new config for Redis pubsub
func NewRedisConfig() RedisConfig {
	return RedisConfig{
		KeepalivePingInterval:     30,
		URL:                       "redis://localhost:6379",
		SentinelDiscoveryInterval: 30,
		TLSVerify:                 false,
		MaxReconnectAttempts:      5,
		DisableCache:              false,
	}
}

func (config *RedisConfig) IsCluster() bool {
	config.mu.RLock()
	defer config.mu.RUnlock()

	return len(config.hosts) > 1 && !config.IsSentinel()
}

func (config *RedisConfig) IsSentinel() bool {
	config.mu.RLock()
	defer config.mu.RUnlock()

	return config.Sentinels != "" || config.sentinelMaster != ""
}

func (config *RedisConfig) Hostnames() []string {
	config.mu.RLock()
	defer config.mu.RUnlock()

	return config.hosts
}

func (config *RedisConfig) Hostname() string {
	config.mu.RLock()
	defer config.mu.RUnlock()

	if config.IsSentinel() {
		return config.sentinelMaster
	} else {
		return config.hosts[0]
	}
}

// TLSClientCertAvailable returns true if both client certificate and key are set
func (config *RedisConfig) TLSClientCertAvailable() bool {
	return config.TLSClientCertPath != "" && config.TLSClientKeyPath != ""
}

// loadCACertPool loads CA certificate pool from file for server verification
func (config *RedisConfig) loadCACertPool() (*x509.CertPool, error) {
	caPEM, err := os.ReadFile(config.TLSCACertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Redis CA certificate: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse Redis CA certificate")
	}

	return certPool, nil
}

func (config *RedisConfig) ToRueidisOptions() (options *rueidis.ClientOption, err error) {
	if config.IsSentinel() {
		options, err = config.parseSentinels()
	} else {
		options, err = parseRedisURL(config.URL)
	}

	if err != nil {
		return nil, err
	}

	config.mu.Lock()
	config.hosts = append([]string{}, options.InitAddress...)
	config.sentinelMaster = options.Sentinel.MasterSet
	config.mu.Unlock()

	options.Dialer.KeepAlive = time.Duration(config.KeepalivePingInterval) * time.Second

	options.ShuffleInit = config.IsCluster()

	if options.TLSConfig != nil {
		options.TLSConfig.InsecureSkipVerify = !config.TLSVerify

		// Load CA certificate for server verification if configured
		if config.TLSCACertPath != "" {
			certPool, err := config.loadCACertPool()
			if err != nil {
				return nil, err
			}
			options.TLSConfig.RootCAs = certPool
		}

		// Load client certificate for mutual TLS if configured
		if config.TLSClientCertAvailable() {
			cert, err := tls.LoadX509KeyPair(config.TLSClientCertPath, config.TLSClientKeyPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load Redis client certificate: %w", err)
			}
			options.TLSConfig.Certificates = []tls.Certificate{cert}
		}
	}

	options.DisableCache = config.DisableCache

	return options, nil
}

func (config *RedisConfig) parseSentinels() (*rueidis.ClientOption, error) {
	config.mu.RLock()
	defer config.mu.RUnlock()

	sentinelMaster, err := url.Parse(config.URL)

	if err != nil {
		return nil, err
	}

	options, err := parseRedisURL(config.Sentinels)

	if err != nil {
		return nil, err
	}

	options.Sentinel.MasterSet = sentinelMaster.Host

	return options, nil
}

func (config *RedisConfig) ToToml() string {
	config.mu.RLock()
	defer config.mu.RUnlock()

	var result strings.Builder

	result.WriteString("# Redis instance URL or master name in case of sentinels usage\n")
	result.WriteString("# or list of URLs if cluster usage\n")
	result.WriteString(fmt.Sprintf("url = \"%s\"\n", config.URL))

	result.WriteString("# Sentinel addresses (comma-separated list)\n")
	result.WriteString(fmt.Sprintf("sentinels = \"%s\"\n", config.Sentinels))

	result.WriteString("# Sentinel discovery interval (seconds)\n")
	result.WriteString(fmt.Sprintf("sentinel_discovery_interval = %d\n", config.SentinelDiscoveryInterval))

	result.WriteString("# Keepalive ping interval (seconds)\n")
	result.WriteString(fmt.Sprintf("keepalive_ping_interval = %d\n", config.KeepalivePingInterval))

	result.WriteString("# Enable TLS Verify\n")
	if config.TLSVerify {
		result.WriteString(fmt.Sprintf("tls_verify = %t\n", config.TLSVerify))
	} else {
		result.WriteString("# tls_verify = true\n")
	}

	result.WriteString("# Path to CA certificate file to verify Redis server\n")
	if config.TLSCACertPath != "" {
		result.WriteString(fmt.Sprintf("tls_ca_cert_path = \"%s\"\n", config.TLSCACertPath))
	} else {
		result.WriteString("# tls_ca_cert_path = \"/path/to/ca.pem\"\n")
	}

	result.WriteString("# Path to client TLS certificate file for mutual TLS\n")
	if config.TLSClientCertPath != "" {
		result.WriteString(fmt.Sprintf("tls_client_cert_path = \"%s\"\n", config.TLSClientCertPath))
	} else {
		result.WriteString("# tls_client_cert_path = \"/path/to/cert.pem\"\n")
	}

	result.WriteString("# Path to client TLS private key file for mutual TLS\n")
	if config.TLSClientKeyPath != "" {
		result.WriteString(fmt.Sprintf("tls_client_key_path = \"%s\"\n", config.TLSClientKeyPath))
	} else {
		result.WriteString("# tls_client_key_path = \"/path/to/key.pem\"\n")
	}

	result.WriteString("# Max number of reconnect attempts\n")
	result.WriteString(fmt.Sprintf("max_reconnect_attempts = %d\n", config.MaxReconnectAttempts))

	result.WriteString("# Disable client-side caching\n")
	if config.DisableCache {
		result.WriteString(fmt.Sprintf("disable_cache = %t\n", config.DisableCache))
	} else {
		result.WriteString("# disable_cache = true\n")
	}

	result.WriteString("\n")

	return result.String()
}

func parseRedisURL(url string) (options *rueidis.ClientOption, err error) {
	urls := strings.Split(url, ",")

	for _, addr := range urls {
		addr = chompTrailingSlashHostname(addr)

		currentOptions, err := rueidis.ParseURL(ensureRedisScheme(addr))

		if err != nil {
			return nil, err
		}

		if options == nil {
			options = &currentOptions
		} else {
			options.InitAddress = append(options.InitAddress, currentOptions.InitAddress...)
		}
	}

	return options, nil
}

// TODO: upstream this change to `rueidis` URL parsing
// as the implementation doesn't tolerate the trailing slash hostnames (`redis-cli` does).
func chompTrailingSlashHostname(url string) string {
	return strings.TrimSuffix(url, "/")
}

func ensureRedisScheme(url string) string {
	if strings.Contains(url, "://") {
		return url
	}

	return "redis://" + url
}
