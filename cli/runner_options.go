package cli

import (
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
		return pubsub.NewSubscriber(h, c.BroadcastAdapter, &c.Redis, &c.HTTPPubSub, &c.NATSPubSub)
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
	return WithBroker(func(h broker.Broadcaster, c *config.Config) (broker.Broker, error) {
		if c.BrokerAdapter == "" {
			return broker.NewLegacyBroker(h), nil
		}

		switch c.BrokerAdapter {
		case "memory":
			b := broker.NewMemoryBroker(h, &c.Broker)
			return b, nil
		default:
			return nil, errorx.IllegalArgument.New("Unsupported broker adapter: %s", c.BrokerAdapter)
		}
	})
}
