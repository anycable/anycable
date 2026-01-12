package enats

import (
	"fmt"
	"strings"
)

// Config represents NATS service configuration
type Config struct {
	Enabled          bool     `toml:"enabled"`
	Debug            bool     `toml:"debug"`
	Trace            bool     `toml:"trace"`
	Name             string   `toml:"name"`
	ServiceAddr      string   `toml:"service_addr"`
	ClusterAddr      string   `toml:"cluster_addr"`
	ClusterName      string   `toml:"cluster_name"`
	GatewayAddr      string   `toml:"gateway_addr"`
	GatewayAdvertise string   `toml:"gateway_advertise"`
	Gateways         []string `toml:"gateways"`
	Routes           []string `toml:"routes"`
	JetStream        bool     `toml:"jetstream"`
	StoreDir         string   `toml:"jetstream_store_dir"`
	// Seconds to wait for JetStream to become ready (can take a lot of time when connecting to a cluster)
	JetStreamReadyTimeout int `toml:"jetstream_ready_timeout"`
	// Maximum message payload size in bytes (default: 1048576 = 1MB)
	MaxPayload int `toml:"max_payload"`
}

func (c Config) ToToml() string {
	var result strings.Builder

	if c.Enabled {
		result.WriteString("enabled = true\n")
	} else {
		result.WriteString("# enabled = true\n")
	}

	result.WriteString("#\n# Verbose  logging settings\n")
	if c.Debug {
		result.WriteString("debug = true\n")
	} else {
		result.WriteString("# debug = true\n")
	}
	if c.Trace {
		result.WriteString("trace = true\n")
	} else {
		result.WriteString("# trace = true\n")
	}

	result.WriteString("#\n# Service name\n")
	result.WriteString(fmt.Sprintf("name = \"%s\"\n", c.Name))

	result.WriteString("#\n# Service address\n")
	result.WriteString(fmt.Sprintf("service_addr = \"%s\"\n", c.ServiceAddr))

	result.WriteString("#\n# Cluster configuration\n#\n")
	if c.ClusterAddr != "" {
		result.WriteString(fmt.Sprintf("cluster_addr = \"%s\"\n", c.ClusterAddr))
	} else {
		result.WriteString("# cluster_addr = \"\"\n")
	}
	if c.ClusterName != "" {
		if c.ClusterAddr == "" {
			result.WriteString(fmt.Sprintf("# cluster_name = \"%s\"\n", c.ClusterName))
		} else {
			result.WriteString(fmt.Sprintf("cluster_name = \"%s\"\n", c.ClusterName))
		}
	} else {
		result.WriteString("# cluster_name = \"\"\n")
	}
	if c.GatewayAddr != "" {
		result.WriteString(fmt.Sprintf("gateway_addr = \"%s\"\n", c.GatewayAddr))
	} else {
		result.WriteString("# gateway_addr = \"\"\n")
	}
	if c.GatewayAdvertise != "" {
		result.WriteString(fmt.Sprintf("gateway_advertise = \"%s\"\n", c.GatewayAdvertise))
	} else {
		result.WriteString("# gateway_advertise = \"\"\n")
	}
	if len(c.Gateways) != 0 {
		result.WriteString(fmt.Sprintf("gateways = [\"%s\"]\n", strings.Join(c.Gateways, "\", \"")))
	} else {
		result.WriteString("# gateways = []\n")
	}
	if len(c.Routes) != 0 {
		result.WriteString(fmt.Sprintf("routes = [\"%s\"]\n", strings.Join(c.Routes, "\", \"")))
	} else {
		result.WriteString("# routes = []\n")
	}

	result.WriteString("#\n# JetStream configuration\n#\n")
	if c.JetStream {
		result.WriteString("jetstream = true\n")
	} else {
		result.WriteString("# jetstream = true\n")
	}

	if c.StoreDir == "" {
		result.WriteString("# jetstream_store_dir = \"\"\n")
	} else {
		result.WriteString(fmt.Sprintf("jetstream_store_dir = \"%s\"\n", c.StoreDir))
	}
	if c.JetStream {
		result.WriteString(fmt.Sprintf("jetstream_ready_timeout = %d\n", c.JetStreamReadyTimeout))
	} else {
		result.WriteString(fmt.Sprintf("# jetstream_ready_timeout = %d\n", c.JetStreamReadyTimeout))
	}

	result.WriteString("#\n# Maximum message payload size\n#\n")
	if c.MaxPayload > 0 {
		result.WriteString(fmt.Sprintf("max_payload = %d\n", c.MaxPayload))
	} else {
		result.WriteString("# max_payload = 1048576\n")
	}

	return result.String()
}
