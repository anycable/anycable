package metrics

// Config contains metrics configuration
type Config struct {
	Log            bool
	LogInterval    int // Deprecated
	RotateInterval int
	LogFormatter   string
	HTTP           string
	Host           string
	Port           int
	Statsd         StatsdConfig
}

// NewConfig creates an empty Config struct
func NewConfig() Config {
	return Config{Statsd: NewStatsdConfig()}
}

// LogEnabled returns true iff any log option is specified
func (c *Config) LogEnabled() bool {
	return c.Log || c.LogFormatterEnabled()
}

// HTTPEnabled returns true iff HTTP is not empty
func (c *Config) HTTPEnabled() bool {
	return c.HTTP != ""
}

// LogFormatterEnabled returns true iff LogFormatter is not empty
func (c *Config) LogFormatterEnabled() bool {
	return c.LogFormatter != ""
}
