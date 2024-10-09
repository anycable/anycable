package server

// SSLConfig contains SSL parameters
type SSLConfig struct {
	CertPath string `toml:"cert_path"`
	KeyPath  string `toml:"key_path"`
}

// NewSSLConfig build a new SSLConfig struct
func NewSSLConfig() SSLConfig {
	return SSLConfig{}
}

// Available returns true iff certificate and private keys are set
func (opts *SSLConfig) Available() bool {
	return opts.CertPath != "" && opts.KeyPath != ""
}
