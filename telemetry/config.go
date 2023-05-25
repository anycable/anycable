package telemetry

type Config struct {
	Token    string
	Endpoint string
}

func NewConfig() *Config {
	return &Config{
		Token:    "phc_fc9VFWdFAAm5gSlCodHq93iaxxnTTKbjOwsWgAS1FMP",
		Endpoint: "https://app.posthog.com",
	}
}
