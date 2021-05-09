package apollo

type Config struct {
	// Where to mount WS handler
	Path string
	// Action Cable channel class name
	Channel string
	// Action Cable channel action name
	Action string
}

func NewConfig() Config {
	return Config{Channel: "GraphqlChannel", Action: "execute"}
}

func (c Config) Enabled() bool {
	return c.Path != ""
}
