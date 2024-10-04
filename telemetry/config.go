package telemetry

import "os"

type Config struct {
	Token    string
	Endpoint string
	Debug    bool
}

var authToken = "secret" // make it overridable during build time

func NewConfig() *Config {
	return &Config{
		Token:    authToken,
		Endpoint: "https://telemetry.anycable.io",
		Debug:    os.Getenv("ANYCABLE_TELEMETRY_DEBUG") == "1",
	}
}
