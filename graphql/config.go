package graphql

type Config struct {
	// Where to mount WS handler
	Path string
	// Action Cable channel class name
	Channel string
	// Action Cable channel action name
	Action string
	// JWT token param name
	JWTParam string
	// For how long to wait for connection_init msg (in seconds)
	IdleTimeout int
}

const (
	// From https://github.com/apollographql/subscriptions-transport-ws/blob/master/src/protocol.ts
	LegacyGraphQLProtocol = "graphql-ws"
)

func GraphqlProtocols() []string {
	return []string{LegacyGraphQLProtocol}
}

func NewConfig() Config {
	return Config{Channel: "GraphqlChannel", Action: "execute", IdleTimeout: 2}
}

func (c Config) Enabled() bool {
	return c.Path != ""
}
