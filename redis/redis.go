package redis

// RedisConfig contains Redis pubsub adapter configuration
type RedisConfig struct {
	// Redis instance URL or master name in case of sentinels usage
	URL string
	// Redis channel to subscribe to
	Channel string
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
}

// NewRedisConfig builds a new config for Redis pubsub
func NewRedisConfig() RedisConfig {
	return RedisConfig{
		KeepalivePingInterval:     30,
		URL:                       "redis://localhost:6379/5",
		Channel:                   "__anycable__",
		SentinelDiscoveryInterval: 30,
		TLSVerify:                 false,
		MaxReconnectAttempts:      5,
	}
}
