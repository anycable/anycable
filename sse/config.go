package sse

// Long-polling configuration
type Config struct {
	Enabled bool
	// Path is the URL path to handle long-polling requests
	Path string
	// List of allowed origins for CORS requests
	// We inherit it from the ws.Config
	AllowedOrigins string
}

// NewConfig creates a new Config with default values.
func NewConfig() Config {
	return Config{
		Enabled: false,
		Path:    "/events",
	}
}
