package ws

// Config contains WebSocket connection configuration.
type Config struct {
	ReadBufferSize    int
	WriteBufferSize   int
	MaxMessageSize    int64
	EnableCompression bool
}

// NewConfig build a new Config struct
func NewConfig() Config {
	return Config{}
}
