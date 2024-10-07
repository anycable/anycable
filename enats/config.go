package enats

// Config represents NATS service configuration
type Config struct {
	Enabled          bool
	Debug            bool
	Trace            bool
	Name             string
	ServiceAddr      string
	ClusterAddr      string
	ClusterName      string
	GatewayAddr      string
	GatewayAdvertise string
	Gateways         []string
	Routes           []string
	JetStream        bool
	StoreDir         string
	// Seconds to wait for JetStream to become ready (can take a lot of time when connecting to a cluster)
	JetStreamReadyTimeout int
}
