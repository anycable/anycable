package config

// SSLOptions contains SSL parameters
type SSLOptions struct {
	CertPath string
	KeyPath  string
}

// Available returns true iff certificate and private keys are set
func (opts *SSLOptions) Available() bool {
	return opts.CertPath != "" && opts.KeyPath != ""
}

// Config contains main application configuration
type Config struct {
	RPCHost              string
	RedisURL             string
	RedisChannel         string
	RedisSentinels       string
	RedisSentinelEnabled bool
	RedisPassword        string
	RedisMasterName      string
	Host                 string
	Port                 int
	Path                 string
	HealthPath           string
	Headers              []string
	SSL                  SSLOptions
	MaxMessageSize       int64
	DisconnectRate       int
	LogLevel             string
	LogFormat            string
	MetricsLog           bool
	MetricsLogInterval   int
	MetricsLogFormatter  string
	MetricsHTTP          string
	MetricsHost          string
	MetricsPort          int
}

// New returns a new empty config
func New() Config {
	config := Config{}
	config.SSL = SSLOptions{}
	return config
}

// MetricsLogEnabled returns true iff MetricsLog is true
func (c *Config) MetricsLogEnabled() bool {
	return c.MetricsLog || c.MetricsLogFormatterEnabled()
}

// MetricsHTTPEnabled returns true iff MetricsHTTP is not empty
func (c *Config) MetricsHTTPEnabled() bool {
	return c.MetricsHTTP != ""
}

// MetricsLogFormatterEnabled returns true iff MetricsLogFormatter is not empty
func (c *Config) MetricsLogFormatterEnabled() bool {
	return c.MetricsLogFormatter != ""
}
