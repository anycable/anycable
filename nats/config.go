package nats

import (
	natsgo "github.com/nats-io/nats.go"
)

type NATSConfig struct {
	Servers              string
	Channel              string
	DontRandomizeServers bool
	MaxReconnectAttempts int
	// Internal channel name for node-to-node broadcasting
	InternalChannel string
}

func NewNATSConfig() NATSConfig {
	return NATSConfig{
		Servers:              natsgo.DefaultURL,
		Channel:              "__anycable__",
		MaxReconnectAttempts: 5,
		InternalChannel:      "__anycable_internal__",
	}
}
