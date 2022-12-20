package nats

import (
	natsgo "github.com/nats-io/nats.go"
)

type NATSConfig struct {
	Servers              string
	Channel              string
	DontRandomizeServers bool
	MaxReconnectAttempts int
}

func NewNATSConfig() NATSConfig {
	return NATSConfig{Servers: natsgo.DefaultURL, Channel: "__anycable__", MaxReconnectAttempts: 5}
}
