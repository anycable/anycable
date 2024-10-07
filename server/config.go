package server

type Config struct {
	Host       string
	Port       int
	MaxConn    int
	HealthPath string
	SSL        SSLConfig
}

func NewConfig() Config {
	return Config{
		Host:       "localhost",
		Port:       8080,
		HealthPath: "/health",
		SSL:        NewSSLConfig(),
	}
}
