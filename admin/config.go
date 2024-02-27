package admin

type Config struct {
	// Secret is used to sign authentication tokens and streams
	Secret string
	// Path under which to mount admin UI and APIs
	Path string
	// Port on which to run admin HTTP server (if 0 then it will be the same as the main server port)
	Port int
	// Enabled define whether admin API is enabled
	Enabled bool
}

// NewConfig returns a new Config with default values
func NewConfig() Config {
	return Config{
		Secret:  "",
		Path:    "/_high_voltage_",
		Enabled: false,
	}
}
