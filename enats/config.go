package enats

// Config represents NATS service configuration
type Config struct {
	Debug            bool
	Trace            bool
	ServiceAddr      string
	ClusterAddr      string
	ClusterName      string
	GatewayAddr      string
	GatewayAdvertise string
	Gateways         []string
	Routes           []string
	JetStream        bool
}
