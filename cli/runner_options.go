package cli

import (
	"log/slog"
	"os"

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

// WithController is an Option to set Runner controller
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
	return WithController(func(m *metrics.Metrics, c *config.Config, l *slog.Logger) (node.Controller, error) {
		if c.RPC.Implementation == "none" {
			return node.NewNullController(l), nil
		}

		return rpc.NewController(m, &c.RPC, l)
	})
}

// WithDisconnector is a an Option to set Runner disconnector
func WithDisconnector(fn disconnectorFactory) Option {
	return func(r *Runner) error {
		r.disconnectorFactory = fn
		return nil
	}
}

// WithBroadcaster is an Option to set Runner broadaster
func WithBroadcasters(fn broadcastersFactory) Option {
	return func(r *Runner) error {
		r.broadcastersFactory = fn
		return nil
	}
}

// WithDefaultBroadcaster is an Option to set Runner subscriber to default broadcaster from config
func WithDefaultBroadcaster() Option {
	return WithBroadcasters(func(h broadcast.Handler, c *config.Config, l *slog.Logger) ([]broadcast.Broadcaster, error) {
		broadcasters := []broadcast.Broadcaster{}
		adapters := c.BroadcastAdapters

		for _, adapter := range adapters {
			switch adapter {
			case "http":
				hb := broadcast.NewHTTPBroadcaster(h, &c.HTTPBroadcast, l)
				broadcasters = append(broadcasters, hb)
			case "redis":
				rb := broadcast.NewLegacyRedisBroadcaster(h, &c.LegacyRedisBroadcast, l)
				broadcasters = append(broadcasters, rb)
			case "redisx":
				rb := broadcast.NewRedisBroadcaster(h, &c.RedisBroadcast, l)
				broadcasters = append(broadcasters, rb)
			case "nats":
				nb := broadcast.NewLegacyNATSBroadcaster(h, &c.NATSBroadcast, l)
				broadcasters = append(broadcasters, nb)
			default:
				return broadcasters, errorx.IllegalArgument.New("Unsupported broadcast adapter: %s", adapter)
			}
		}

		return broadcasters, nil
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
	return WithSubscriber(func(h pubsub.Handler, c *config.Config, l *slog.Logger) (pubsub.Subscriber, error) {
		switch c.PubSubAdapter {
		case "":
			return pubsub.NewLegacySubscriber(h), nil
		case "redis":
			return pubsub.NewRedisSubscriber(h, &c.RedisPubSub, l)
		case "nats":
			return pubsub.NewNATSSubscriber(h, &c.NATSPubSub, l)
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
	return WithBroker(func(br broker.Broadcaster, c *config.Config, l *slog.Logger) (broker.Broker, error) {
		if c.Broker.Adapter == "" {
			return broker.NewLegacyBroker(br), nil
		}

		switch c.Broker.Adapter {
		case "memory":
			b := broker.NewMemoryBroker(br, &c.Broker)
			return b, nil
		case "nats":
			// TODO: Figure out a better place for this hack.
			// We don't want to enable JetStream by default (if NATS is used only for pub/sub),
			// currently, we only need it when NATS is used as a broker.
			c.EmbeddedNats.JetStream = true
			b := broker.NewNATSBroker(br, &c.Broker, &c.NATS, l)
			return b, nil
		default:
			return nil, errorx.IllegalArgument.New("Unsupported broker adapter: %s", c.Broker.Adapter)
		}
	})
}

// WithTelemetry enables AnyCable telemetry unless ANYCABLE_DISABLE_TELEMETRY is set
func WithTelemetry() Option {
	return func(r *Runner) error {
		r.telemetryEnabled = os.Getenv("ANYCABLE_DISABLE_TELEMETRY") != "true"
		return nil
	}
}

// WithLogger set ups a logger for the AnyCable app
func WithLogger(logger *slog.Logger) Option {
	return func(r *Runner) error {
		r.log = logger
		return nil
	}
}
