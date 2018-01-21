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
	RPCHost        string
	RedisURL       string
	RedisChannel   string
	Host           string
	Port           int
	Path           string
	Headers        []string
	SSL            SSLOptions
	DisconnectRate int
	LogLevel       string
	LogFormat      string
}

// New returns a new empty config
func New() Config {
	config := Config{}
	config.SSL = SSLOptions{}
	return config
}
