package lp

const (
	defaultMaxBodySize = 65536 // 64 kB
)

// Long-polling configuration
type Config struct {
	Enabled bool
	// Path is the URL path to handle long-polling requests
	Path string
	// MaxBodySize is the maximum size of the request body in bytes.
	MaxBodySize int
	// FlushInterval is the duration before flushing buffered data to the client in milliseconds.
	FlushInterval int
	// PollInterval is the default polling duration in seconds
	PollInterval int
	// KeepaliveTimeout is the duration for how long to consider the virtual connection to be alive between poll requests (in seconds)
	KeepaliveTimeout int
	// List of allowed origins for CORS requests
	// We inherit it from the ws.Config
	AllowedOrigins string
}

// NewConfig creates a new Config with default values.
func NewConfig() Config {
	return Config{
		Enabled:          false,
		Path:             "/lp",
		MaxBodySize:      defaultMaxBodySize,
		FlushInterval:    500,
		PollInterval:     15,
		KeepaliveTimeout: 5,
	}
}
