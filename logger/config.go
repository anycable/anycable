package logger

type Config struct {
	LogLevel  string
	LogFormat string
	Debug     bool
}

func NewConfig() Config {
	return Config{
		LogLevel:  "info",
		LogFormat: "text",
	}
}
