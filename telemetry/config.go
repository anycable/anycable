package telemetry

import "os"

type Config struct {
	Token       string
	Endpoint    string
	CustomProps map[string]string
	Debug       bool
}

var auth = "" // provide a secret token during the build time to enable tracking

func NewConfig() *Config {
	return &Config{
		Token:       auth,
		Endpoint:    "https://telemetry.anycable.io",
		Debug:       os.Getenv("ANYCABLE_TELEMETRY_DEBUG") == "1",
		CustomProps: map[string]string{},
	}
}
