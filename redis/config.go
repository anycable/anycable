package redis

import (
	"time"
	"net/url"

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
	// A parsed URL of Sentinel Master
	sentinelMaster *url.URL
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

func (config *RedisConfig) ToRueidisOptions() (*rueidis.ClientOption, error) {
  options, err := rueidis.ParseURL(config.URL)

	if err != nil {
		return nil, err
	}

	if config.IsSentinel() {
		config.sentinelMaster, _ = url.Parse(options.InitAddress[0])

	  options, err := rueidis.ParseURL(config.Sentinels)

		if err != nil {
			return nil, err
		}

		options.Sentinel.MasterSet = config.sentinelMaster.Host
	}

	config.uris = make([]*url.URL, len(options.InitAddress))
	for i, addr := range options.InitAddress {
		uri, _ := url.Parse(addr)
		config.uris[i] = uri
	}

	options.Dialer.KeepAlive = time.Duration(config.KeepalivePingInterval) * time.Second

	options.ShuffleInit = config.IsCluster()

	if options.TLSConfig != nil {
		options.TLSConfig.InsecureSkipVerify = !config.TLSVerify
	}

	return &options, nil
}
