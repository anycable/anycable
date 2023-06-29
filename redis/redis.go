package redis

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/redis/rueidis"
)

// RedisConfig contains Redis pubsub adapter configuration
type RedisConfig struct {
	// Redis instance URL or master name in case of sentinels usage
	// or list of URLs if cluster usage
	URL string
	// Redis channel to subscribe to (legacy pub/sub)
	Channel string
	// Redis stream consumer group name
	Group string
	// Redis stream read wait time in milliseconds
	StreamReadBlockMilliseconds int64
	// Internal channel name for node-to-node broadcasting
	InternalChannel string
	// List of Redis Sentinel addresses
	Sentinels string
	// Redis Sentinel discovery interval (seconds)
	SentinelDiscoveryInterval int
	// Redis keepalive ping interval (seconds)
	KeepalivePingInterval int
	// Whether to check server's certificate for validity (in case of rediss:// protocol)
	TLSVerify bool
	// Max number of reconnect attempts
	MaxReconnectAttempts int

	// List of parsed URLs
	uris []*url.URL

	// Parsed sentinels URLs
	sentinels []*url.URL
}

// NewRedisConfig builds a new config for Redis pubsub
func NewRedisConfig() RedisConfig {
	return RedisConfig{
		KeepalivePingInterval:       30,
		URL:                         "redis://localhost:6379/5",
		Channel:                     "__anycable__",
		Group:                       "bx",
		StreamReadBlockMilliseconds: 2000,
		InternalChannel:             "__anycable_internal__",
		SentinelDiscoveryInterval:   30,
		TLSVerify:                   false,
		MaxReconnectAttempts:        5,
	}
}

func (config *RedisConfig) IsSentinel() bool {
	return config.Sentinels != ""
}

func (config *RedisConfig) IsCluster() bool {
	return len(config.uris) > 1
}

func (config *RedisConfig) HasAuth() bool {
	uri := config.uris[0]

	if config.IsSentinel() {
		uri = config.sentinels[0]
	}

	_, hasPassword := uri.User.Password()

	return hasPassword
}

func (config *RedisConfig) HasTLS() bool {
	uri := config.uris[0]

	return uri.Scheme == "rediss"
}

func (config *RedisConfig) ToTLSConfig() *tls.Config {
	return &tls.Config{InsecureSkipVerify: !config.TLSVerify} // nolint:gosec
}

func (config *RedisConfig) Username() string {
	uri := config.uris[0]

	if config.IsSentinel() {
		uri = config.sentinels[0]
	}

	return uri.User.Username()
}

func (config *RedisConfig) Password() string {
	uri := config.uris[0]

	if config.IsSentinel() {
		uri = config.sentinels[0]
	}

	password, _ := uri.User.Password()

	return password
}

func (config *RedisConfig) Hostname() string {
	uri := config.uris[0]

	return uri.Host
}

func (config *RedisConfig) Hostnames() []string {
	hostnames := make([]string, len(config.uris))

	for i, uri := range config.uris {
		hostnames[i] = uri.Host
	}

	return hostnames
}

func (config *RedisConfig) SentinelHostnames() []string {
	if !config.IsSentinel() {
		return nil
	}

	hostnames := make([]string, len(config.sentinels))

	for i, uri := range config.sentinels {
		hostnames[i] = uri.Host
	}

	return hostnames
}

// Must be called before accessing URIs and sentinels data
func (config *RedisConfig) Parse() error {
	urls := strings.Split(config.URL, ",")

	uris := make([]*url.URL, len(urls))

	for i, addr := range urls {
		// parse URL and check if it is correct
		uri, err := url.Parse(addr)

		if err != nil {
			return err
		}

		uris[i] = uri
	}

	config.uris = uris

	if config.Sentinels != "" {
		sentinelHostnames := strings.Split(config.Sentinels, ",")

		sentinels := make([]*url.URL, len(sentinelHostnames))

		for i, addr := range sentinelHostnames {
			uri, err := url.Parse(fmt.Sprintf("redis://%s", addr))

			if err != nil {
				return err
			}

			sentinels[i] = uri
		}

		config.sentinels = sentinels
	}

	return nil
}

func (config *RedisConfig) ToRueidisOptions() (*rueidis.ClientOption, error) {
	err := config.Parse()

	if err != nil {
		return nil, err
	}

	keepalive := time.Duration(config.KeepalivePingInterval) * time.Second

	options := &rueidis.ClientOption{
		Dialer: net.Dialer{
			KeepAlive: keepalive,
		},
	}

	if config.IsCluster() { //nolint:gocritic
		options.InitAddress = config.Hostnames()
		options.ShuffleInit = true
	} else if config.IsSentinel() {
		options.InitAddress = config.SentinelHostnames()
		options.Sentinel = rueidis.SentinelOption{
			MasterSet: config.Hostname(),
		}
	} else {
		options.InitAddress = config.Hostnames()
	}

	if config.HasAuth() {
		options.Username = config.Username()
		options.Password = config.Password()
	}

	if config.HasTLS() {
		options.TLSConfig = config.ToTLSConfig()
	}

	return options, nil
}
