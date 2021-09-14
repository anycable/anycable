package rails

type Config struct {
	TurboRailsKey string
	CableReadyKey string
}

func NewConfig() Config {
	return Config{}
}
