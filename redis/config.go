package redis

import (
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

	// List of hosts to connect
	hosts []string
	// Sentinel Master host to connect
	sentinelMaster string
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

func (config *RedisConfig) IsCluster() bool {
	return len(config.hosts) > 1 && !config.IsSentinel()
}

func (config *RedisConfig) IsSentinel() bool {
	return config.Sentinels != "" || config.sentinelMaster != ""
}

func (config *RedisConfig) Hostnames() []string {
	return config.hosts
}

func (config *RedisConfig) Hostname() string {
	if config.IsSentinel() {
		return config.sentinelMaster
	} else {
		return config.hosts[0]
	}
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

	config.hosts = options.InitAddress
	config.sentinelMaster = options.Sentinel.MasterSet

	options.Dialer.KeepAlive = time.Duration(config.KeepalivePingInterval) * time.Second

	options.ShuffleInit = config.IsCluster()

	if options.TLSConfig != nil {
		options.TLSConfig.InsecureSkipVerify = !config.TLSVerify
	}

	return options, nil
}

func (config *RedisConfig) parseSentinels() (*rueidis.ClientOption, error) {
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
