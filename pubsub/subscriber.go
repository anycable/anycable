package pubsub

import (
	"fmt"

	"github.com/anycable/anycable-go/node"
)

// Subscriber is responsible for receiving broadcast messages
// and sending them to hub
type Subscriber interface {
	Start() error
	Shutdown() error
}

// NewSubscriber creates an instance of the provided adapter
func NewSubscriber(node node.AppNode, adapter string, redis *RedisConfig, http *HTTPConfig) (Subscriber, error) {
	switch adapter {
	case "redis":
		return NewRedisSubscriber(node, redis), nil
	case "http":
		return NewHTTPSubscriber(node, http), nil
	}

	return nil, fmt.Errorf("Unknown adapter type: %s", adapter)
}
