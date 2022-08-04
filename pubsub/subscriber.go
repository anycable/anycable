package pubsub

import (
	"fmt"
)

// Subscriber is responsible for receiving broadcast messages
// and sending them to hub
type Subscriber interface {
	Start(done chan (error)) error
	Shutdown() error
}

type Handler interface {
	HandlePubSub(json []byte)
}

// NewSubscriber creates an instance of the provided adapter
func NewSubscriber(node Handler, adapter string, redis *RedisConfig, http *HTTPConfig, nats *NATSConfig) (Subscriber, error) {
	switch adapter {
	case "redis":
		return NewRedisSubscriber(node, redis), nil
	case "http":
		return NewHTTPSubscriber(node, http), nil
	case "nats":
		return NewNATSSubscriber(node, nats), nil
	}

	return nil, fmt.Errorf("Unknown adapter type: %s", adapter)
}
