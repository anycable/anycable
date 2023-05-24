package rails

type Config struct {
	TurboRailsKey       string
	TurboRailsClearText bool
	CableReadyKey       string
	CableReadyClearText bool
}

func NewConfig() Config {
	return Config{}
}
