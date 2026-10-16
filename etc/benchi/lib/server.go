package lib

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/anycable/anycable-go/cli"
	"github.com/anycable/anycable-go/config"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/node"
)

// ServerConfig configures the embedded AnyCable server built by BuildServer.
// The zero value yields v1 defaults: public-streams mode, skip-auth, RPC
// implementation "none", LegacyBroker (zero-history), default subscriber, and
// a discard logger. Public fields exist so future scenarios can swap
// components without modifying the lib package.
type ServerConfig struct {
	// Logger overrides the default discard logger. Leave unset to satisfy
	// R13 (no server log output on stdout/stderr); set to a real logger for
	// development or when capturing server-side activity in a scenario test.
	Logger *slog.Logger

	// BrokerAdapter selects the broker. Accepted values mirror the root
	// --broker flag: "" or "none" (LegacyBroker — zero history, the v1
	// default), "memory" (in-process history), "nats" (external).
	BrokerAdapter string

	// ExtraOptions are appended to the option list passed to cli.NewRunner
	// after the v1 defaults. Use to attach broadcasters or swap the
	// subscriber when adding a new scenario.
	ExtraOptions []cli.Option

	// RPCHost, when non-empty, switches the server from the v1 public-streams
	// + skip-auth mode to HTTP RPC mode: Connect, Command and Disconnect calls
	// go to RPCHost (see RPCServer for an in-process fake).
	RPCHost string

	// Configure, when non-nil, runs after all defaults are applied and
	// before the runner is built. Use it to tune node settings (ping
	// interval, disconnect queue, RPC concurrency) for a scenario.
	Configure func(*config.Config)
}

// Server wraps cli.Embedded plus a loopback httptest.Server that exposes the
// WebSocket and HTTP broadcast handlers at known URLs.
type Server struct {
	embedded   *cli.Embedded
	runner     *cli.Runner
	disconnQ   *node.DisconnectQueue
	testServer *httptest.Server
	once       sync.Once
}

// BuildServer constructs and starts an embedded AnyCable server in
// public-streams + skip-auth mode and mounts its WebSocket and HTTP broadcast
// handlers on a loopback httptest.Server.
//
// Only one Server per process is supported in v1: cli.NewRunner mutates
// package-level globals in the server package (Host, MaxConn, Logger, SSL),
// so concurrent BuildServer calls would race. Callers must Shutdown the
// previous Server before building another.
func BuildServer(cfg ServerConfig) (*Server, error) {
	c := cli.NewConfig()
	c.SkipAuth = true
	c.RPC.Implementation = "none"
	c.Streams.Public = true
	// LegacyBroker rejects presence operations; the default Presence=true
	// would emit error logs on every client disconnect.
	c.Streams.Presence = false
	c.Broker.Adapter = cfg.BrokerAdapter
	// Single-process embedded server: only the HTTP broadcaster matters.
	// Stripping "redis" from BroadcastAdapters (default ["http", "redis"])
	// drops the LegacyRedisBroadcaster — both removing the Redis dependency
	// and eliminating the post-subscribe race where the Redis pubsub had not
	// yet subscribed to the stream when the first broadcast arrived.
	c.BroadcastAdapters = []string{"http"}
	c.PubSubAdapter = "none"

	if cfg.RPCHost != "" {
		c.SkipAuth = false
		c.RPC.Implementation = "http"
		c.RPC.Host = cfg.RPCHost
		c.Streams.Public = false
	}

	if cfg.Configure != nil {
		cfg.Configure(c)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	// Build the default disconnect queue here to keep a handle on it:
	// the disconnect_queue_size gauge refreshes only with the node stats.
	var disconnQ *node.DisconnectQueue
	opts := []cli.Option{
		cli.WithDisconnector(func(n *node.Node, c *config.Config, l *slog.Logger) (node.Disconnector, error) {
			if c.DisconnectorDisabled {
				return node.NewNoopDisconnector(), nil
			}
			disconnQ = node.NewDisconnectQueue(n, &c.DisconnectQueue, l)
			return disconnQ, nil
		}),
		cli.WithName("benchi"),
		cli.WithDefaultRPCController(),
		cli.WithDefaultBroker(),
		cli.WithDefaultSubscriber(),
		cli.WithLogger(logger),
	}
	opts = append(opts, cfg.ExtraOptions...)

	runner, err := cli.NewRunner(c, opts)
	if err != nil {
		return nil, err
	}

	embedded, err := runner.Embed()
	if err != nil {
		return nil, err
	}

	wsHandler, err := embedded.WebSocketHandler()
	if err != nil {
		_ = embedded.Shutdown(context.Background())
		return nil, err
	}

	broadcastHandler, err := embedded.HTTPBroadcastHandler()
	if err != nil {
		_ = embedded.Shutdown(context.Background())
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle("/cable", wsHandler)
	mux.Handle("/broadcast", broadcastHandler)

	ts := httptest.NewUnstartedServer(mux)
	ts.Start()

	return &Server{embedded: embedded, runner: runner, disconnQ: disconnQ, testServer: ts}, nil
}

// DisconnectQueueSize returns the current number of sessions waiting in the
// disconnect queue. Unlike the disconnect_queue_size gauge, it is never stale.
// Returns 0 when the disconnector is disabled or replaced via ExtraOptions.
func (s *Server) DisconnectQueueSize() int {
	if s.disconnQ == nil {
		return 0
	}
	return s.disconnQ.Size()
}

// Run is a no-op kept for symmetry with the brainstorm signature. The
// embedded runner started node, broker, subscriber, and metrics during
// BuildServer.
func (s *Server) Run() error { return nil }

// Shutdown stops the embedded runner and the loopback HTTP server. The first
// call propagates ctx into cli.Embedded.Shutdown; subsequent calls are no-ops
// and return nil — the scenario teardown path may call Shutdown more than
// once (defer + explicit) and must not panic.
func (s *Server) Shutdown(ctx context.Context) error {
	var err error
	s.once.Do(func() {
		err = s.embedded.Shutdown(ctx)
		s.testServer.Close()
	})
	return err
}

// Gauge returns the current value of the server metrics gauge with the given
// name (e.g. "disconnect_queue_size", "clients_num", "rpc_pending_num").
// Returns 0 for unknown gauges. Gauges refreshed by the node stats collector
// lag by up to App.StatsRefreshInterval seconds.
func (s *Server) Gauge(name string) uint64 {
	m, ok := s.runner.Instrumenter().(*metrics.Metrics)
	if !ok || m == nil {
		return 0
	}
	g := m.Gauge(name)
	if g == nil {
		return 0
	}
	return g.Value()
}

// Counter returns the current total of the server metrics counter with the
// given name (e.g. "rpc_canceled_total", "rpc_call_total"). Returns 0 for
// unknown counters.
func (s *Server) Counter(name string) uint64 {
	m, ok := s.runner.Instrumenter().(*metrics.Metrics)
	if !ok || m == nil {
		return 0
	}
	c := m.Counter(name)
	if c == nil {
		return 0
	}
	return c.Value()
}

// BroadcastURL returns the URL the publisher should POST broadcasts to.
func (s *Server) BroadcastURL() string {
	return s.testServer.URL + "/broadcast"
}

// WebSocketURL returns the URL clients should dial to subscribe via WebSocket.
func (s *Server) WebSocketURL() string {
	return "ws://" + strings.TrimPrefix(s.testServer.URL, "http://") + "/cable"
}
