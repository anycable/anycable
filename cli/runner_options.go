package cli

import (
	"github.com/anycable/anycable-go/broadcast"
	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/config"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/anycable/anycable-go/rpc"
	"github.com/joomcode/errorx"
)

// Option represents a Runner configuration function
type Option func(*Runner) error

// WithName is an Option to set Runner name
func WithName(name string) Option {
	return func(r *Runner) error {
		r.name = name
		return nil
	}
}

// WithName is an Option to set Runner controller
func WithController(fn controllerFactory) Option {
	return func(r *Runner) error {
		if r.controllerFactory != nil {
			return errorx.IllegalArgument.New("Controller has been already assigned")
		}
		r.controllerFactory = fn
		return nil
	}
}

// WithDefaultRPCController is an Option to set Runner controller to default rpc.Controller
func WithDefaultRPCController() Option {
	return WithController(func(m *metrics.Metrics, c *config.Config) (node.Controller, error) {
		return rpc.NewController(m, &c.RPC), nil
	})
}

// WithBroadcaster is an Option to set Runner broadaster
func WithBroadcaster(fn broadcasterFactory) Option {
	return func(r *Runner) error {
		if r.broadcasterFactory != nil {
			return errorx.IllegalArgument.New("Broadcaster has been already assigned")
		}
		r.broadcasterFactory = fn
		return nil
	}
}

// WithDefaultBroadcaster is an Option to set Runner subscriber to default broadcaster from config
func WithDefaultBroadcaster() Option {
	return WithBroadcaster(func(h broadcast.Handler, c *config.Config) (broadcast.Broadcaster, error) {
		switch c.BroadcastAdapter {
		case "http":
			return broadcast.NewHTTPBroadcaster(h, &c.HTTPBroadcast), nil
		case "redis":
			return broadcast.NewRedisBroadcaster(h, &c.RedisBroadcast), nil
		case "nats":
			return broadcast.NewNATSBroadcaster(h, &c.NATSBroadcast), nil
		default:
			return nil, errorx.IllegalArgument.New("Unsupported broadcast adapter: %s", c.BroadcastAdapter)
		}
	})
}

// WithSubscriber is an Option to set Runner subscriber
func WithSubscriber(fn subscriberFactory) Option {
	return func(r *Runner) error {
		if r.subscriberFactory != nil {
			return errorx.IllegalArgument.New("Subscriber has been already assigned")
		}
		r.subscriberFactory = fn
		return nil
	}
}

// WithDefaultSubscriber is an Option to set Runner subscriber to pubsub.NewSubscriber
func WithDefaultSubscriber() Option {
	return WithSubscriber(func(h pubsub.Handler, c *config.Config) (pubsub.Subscriber, error) {
		if c.PubSubAdapter == "" {
			return pubsub.NewLegacySubscriber(h), nil
		}

		return nil, errorx.IllegalArgument.New("Unsupported subscriber adapter: %s", c.PubSubAdapter)
	})
}

// WithShutdowable adds a new shutdownable instance to be shutdown at server stop
func WithShutdownable(instance Shutdownable) Option {
	return func(r *Runner) error {
		r.shutdownables = append(r.shutdownables, instance)
		return nil
	}
}

// WithBroker is an Option to set Runner broker
func WithBroker(fn brokerFactory) Option {
	return func(r *Runner) error {
		if r.brokerFactory != nil {
			return errorx.IllegalArgument.New("Broker has been already assigned")
		}
		r.brokerFactory = fn
		return nil
	}
}

// WithWebSocketHandler is an Option to set a custom websocket handler
func WithWebSocketHandler(fn websocketHandler) Option {
	return func(r *Runner) error {
		r.websocketHandlerFactory = fn
		return nil
	}
}

// WithWebSocketEndpoint is an Option to set a custom websocket handler at
// the specified path
func WithWebSocketEndpoint(path string, fn websocketHandler) Option {
	return func(r *Runner) error {
		r.websocketEndpoints[path] = fn
		return nil
	}
}

// WithDefaultBroker is an Option to set Runner broker to default broker from config
func WithDefaultBroker() Option {
	return WithBroker(func(br broker.Broadcaster, c *config.Config) (broker.Broker, error) {
		if c.BrokerAdapter == "" {
			return broker.NewLegacyBroker(br), nil
		}

		switch c.BrokerAdapter {
		case "memory":
			b := broker.NewMemoryBroker(br, &c.Broker)
			return b, nil
		default:
			return nil, errorx.IllegalArgument.New("Unsupported broker adapter: %s", c.BrokerAdapter)
		}
	})
}
