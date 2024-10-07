package ws

// Config contains WebSocket connection configuration.
type Config struct {
	Paths             []string
	ReadBufferSize    int
	WriteBufferSize   int
	MaxMessageSize    int64
	EnableCompression bool
	AllowedOrigins    string
}

// NewConfig build a new Config struct
func NewConfig() Config {
	return Config{Paths: []string{"/cable"}, ReadBufferSize: 1024, WriteBufferSize: 1024, MaxMessageSize: 65536}
}
