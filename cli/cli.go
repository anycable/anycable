package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/anycable/anycable-go/broadcast"
	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/config"
	"github.com/anycable/anycable-go/enats"
	"github.com/anycable/anycable-go/identity"
	"github.com/anycable/anycable-go/logger"
	metricspkg "github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mrb"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/anycable/anycable-go/router"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/sse"
	"github.com/anycable/anycable-go/streams"
	"github.com/anycable/anycable-go/telemetry"
	"github.com/anycable/anycable-go/utils"
	"github.com/anycable/anycable-go/version"
	"github.com/anycable/anycable-go/ws"
	"github.com/gorilla/websocket"
	"github.com/joomcode/errorx"

	"go.uber.org/automaxprocs/maxprocs"
)

type controllerFactory = func(*metricspkg.Metrics, *config.Config, *slog.Logger) (node.Controller, error)
type disconnectorFactory = func(*node.Node, *config.Config, *slog.Logger) (node.Disconnector, error)
type broadcastersFactory = func(broadcast.Handler, *config.Config, *slog.Logger) ([]broadcast.Broadcaster, error)
type brokerFactory = func(broker.Broadcaster, *config.Config, *slog.Logger) (broker.Broker, error)
type subscriberFactory = func(pubsub.Handler, *config.Config, *slog.Logger) (pubsub.Subscriber, error)
type websocketHandler = func(*node.Node, *config.Config, *slog.Logger) (http.Handler, error)

type Shutdownable interface {
	Shutdown(ctx context.Context) error
}

type Runner struct {
	options []Option

	name   string
	config *config.Config
	log    *slog.Logger

	controllerFactory       controllerFactory
	disconnectorFactory     disconnectorFactory
	subscriberFactory       subscriberFactory
	brokerFactory           brokerFactory
	websocketHandlerFactory websocketHandler

	broadcastersFactory broadcastersFactory
	websocketEndpoints  map[string]websocketHandler

	router           *router.RouterController
	metrics          *metricspkg.Metrics
	telemetryEnabled bool

	errChan       chan error
	shutdownables []Shutdownable
}

// NewRunner creates returns new Runner structure
func NewRunner(c *config.Config, options []Option) (*Runner, error) {
	r := &Runner{
		options:            options,
		config:             c,
		shutdownables:      []Shutdownable{},
		websocketEndpoints: make(map[string]websocketHandler),
		errChan:            make(chan error),
	}

	err := r.checkAndSetDefaults()
	if err != nil {
		return nil, err
	}

	return r, nil
}

// checkAndSetDefaults applies passed options and checks that all required fields are set
func (r *Runner) checkAndSetDefaults() error {
	for _, o := range r.options {
		err := o(r)
		if err != nil {
			return err
		}
	}

	if r.log == nil {
		_, err := logger.InitLogger(r.config.Log.LogFormat, r.config.Log.LogLevel)
		if err != nil {
			return errorx.Decorate(err, "failed to initialize default logger")
		}

		r.log = slog.With("nodeid", r.config.ID)
	}

	err := r.config.LoadPresets(r.log)

	if err != nil {
		return errorx.Decorate(err, "failed to load configuration presets")
	}

	server.SSL = &r.config.Server.SSL
	server.Host = r.config.Server.Host
	server.MaxConn = r.config.Server.MaxConn
	server.Logger = r.log

	if r.name == "" {
		return errorx.AssertionFailed.New("Name is blank, specify WithName()")
	}

	if r.controllerFactory == nil {
		return errorx.AssertionFailed.New("Controller is blank, specify WithController()")
	}

	if r.brokerFactory == nil {
		return errorx.AssertionFailed.New("Broker is blank, specify WithBroker()")
	}

	if r.subscriberFactory == nil {
		return errorx.AssertionFailed.New("Subscriber is blank, specify WithSubscriber()")
	}

	if r.disconnectorFactory == nil {
		r.disconnectorFactory = r.defaultDisconnector
	}

	if r.websocketHandlerFactory == nil {
		r.websocketHandlerFactory = r.defaultWebSocketHandler
	}

	metrics, err := r.initMetrics(&r.config.Metrics)

	if err != nil {
		return errorx.Decorate(err, "failed to initialize metrics writer")
	}

	r.metrics = metrics

	return nil
}

// Run starts the instance
func (r *Runner) Run() error {
	numProcs := r.setMaxProcs()
	r.announceDebugMode()

	mrubySupport := r.initMRuby()

	r.log.Info(fmt.Sprintf("Starting %s %s%s (pid: %d, open file limit: %s, gomaxprocs: %d)", r.name, version.Version(), mrubySupport, os.Getpid(), utils.OpenFileLimit(), numProcs))

	if r.config.IsPublic() {
		r.log.Warn("Server is running in the public mode")
	}

	appNode, err := r.runNode()

	if err != nil {
		return err
	}

	wsServer, err := server.ForPort(strconv.Itoa(r.config.Server.Port))
	if err != nil {
		return errorx.Decorate(err, "failed to initialize WebSocket server at %s:%d", r.config.Server.Host, r.config.Server.Port)
	}

	wsHandler, err := r.websocketHandlerFactory(appNode, r.config, r.log)
	if err != nil {
		return errorx.Decorate(err, "failed to initialize WebSocket handler")
	}

	for _, path := range r.config.WS.Paths {
		wsServer.SetupHandler(path, wsHandler)
		r.log.Info(fmt.Sprintf("Handle WebSocket connections at %s%s", wsServer.Address(), path))
	}

	for path, handlerFactory := range r.websocketEndpoints {
		handler, err := handlerFactory(appNode, r.config, r.log)
		if err != nil {
			return errorx.Decorate(err, "failed to initialize WebSocket handler for %s", path)
		}
		wsServer.SetupHandler(path, handler)
	}

	wsServer.SetupHandler(r.config.Server.HealthPath, http.HandlerFunc(server.HealthHandler))
	r.log.Info(fmt.Sprintf("Handle health requests at %s%s", wsServer.Address(), r.config.Server.HealthPath))

	if r.config.SSE.Enabled {
		r.log.Info(
			fmt.Sprintf("Handle SSE requests at %s%s",
				wsServer.Address(), r.config.SSE.Path),
		)

		sseHandler, err := r.defaultSSEHandler(appNode, wsServer.ShutdownCtx(), r.config)

		if err != nil {
			return errorx.Decorate(err, "failed to initialize SSE handler")
		}

		wsServer.SetupHandler(r.config.SSE.Path, sseHandler)
	}

	go r.startWSServer(wsServer)
	go r.metrics.Run() // nolint:errcheck

	// We MUST first stop the server (=stop accepting new connections), then gracefully disconnect active clients
	r.shutdownables = append([]Shutdownable{wsServer}, r.shutdownables...)
	r.setupSignalHandlers()

	// Wait for an error (or none)
	return <-r.errChan
}

func (r *Runner) runNode() (*node.Node, error) {
	metrics := r.metrics

	r.shutdownables = append(r.shutdownables, metrics)

	controller, err := r.newController(metrics)
	if err != nil {
		return nil, err
	}

	appNode := node.NewNode(
		&r.config.App,
		node.WithController(controller),
		node.WithInstrumenter(metrics),
		node.WithLogger(r.log),
		node.WithID(r.config.ID),
	)

	if r.telemetryEnabled {
		telemetryConfig := telemetry.NewConfig()
		if customTelemetryUrl := os.Getenv("ANYCABLE_TELEMETRY_URL"); customTelemetryUrl != "" {
			telemetryConfig.Endpoint = customTelemetryUrl
		}
		tracker := telemetry.NewTracker(metrics, r.config, telemetryConfig)

		r.log.With("context", "telemetry").Info(tracker.Announce())
		go tracker.Collect()

		r.shutdownables = append(r.shutdownables, tracker)
	}

	subscriber, err := r.subscriberFactory(appNode, r.config, r.log)

	if err != nil {
		return nil, errorx.Decorate(err, "couldn't configure pub/sub")
	}

	appBroker, err := r.brokerFactory(subscriber, r.config, r.log)
	if err != nil {
		return nil, errorx.Decorate(err, "failed to initialize broker")
	}

	if appBroker != nil {
		r.log.Info(appBroker.Announce())
		appNode.SetBroker(appBroker)
	}

	disconnector, err := r.disconnectorFactory(appNode, r.config, r.log)
	if err != nil {
		return nil, errorx.Decorate(err, "failed to initialize disconnector")
	}

	go disconnector.Run() // nolint:errcheck
	appNode.SetDisconnector(disconnector)

	if r.config.EmbeddedNats.Enabled {
		service, enatsErr := r.embedNATS(&r.config.EmbeddedNats)

		if enatsErr != nil {
			return nil, errorx.Decorate(enatsErr, "failed to start embedded NATS server")
		}

		desc := service.Description()

		if desc != "" {
			desc = fmt.Sprintf(" (%s)", desc)
		}

		r.log.Info(fmt.Sprintf("Embedded NATS server started: %s%s", r.config.EmbeddedNats.ServiceAddr, desc))

		r.shutdownables = append(r.shutdownables, service)
	}

	err = appNode.Start()

	if err != nil {
		return nil, errorx.Decorate(err, "failed to initialize application")
	}

	err = subscriber.Start(r.errChan)
	if err != nil {
		return nil, errorx.Decorate(err, "failed to start subscriber")
	}

	if appBroker != nil {
		err = appBroker.Start(r.errChan)
		if err != nil {
			return nil, errorx.Decorate(err, "failed to start broker")
		}
	}

	r.shutdownables = append(r.shutdownables, subscriber)

	if r.broadcastersFactory != nil {
		broadcasters, berr := r.broadcastersFactory(appNode, r.config, r.log)

		if berr != nil {
			return nil, errorx.Decorate(err, "couldn't configure broadcasters")
		}

		for _, broadcaster := range broadcasters {
			err = broadcaster.Start(r.errChan)
			if err != nil {
				return nil, errorx.Decorate(err, "failed to start broadcaster")
			}

			r.shutdownables = append(r.shutdownables, broadcaster)
		}
	}

	err = controller.Start()
	if err != nil {
		return nil, errorx.Decorate(err, "failed to initialize RPC controller")
	}

	r.shutdownables = append([]Shutdownable{appNode, appBroker}, r.shutdownables...)

	r.announceGoPools()
	return appNode, nil
}

func (r *Runner) setMaxProcs() int {
	// See https://github.com/uber-go/automaxprocs/issues/18
	nopLog := func(string, ...interface{}) {}
	maxprocs.Set(maxprocs.Logger(nopLog)) // nolint:errcheck

	return runtime.GOMAXPROCS(0)
}

func (r *Runner) announceDebugMode() {
	if r.config.Log.Debug {
		r.log.Debug("🔧 🔧 🔧 Debug mode is on 🔧 🔧 🔧")
	}
}

func (r *Runner) initMetrics(c *metricspkg.Config) (*metricspkg.Metrics, error) {
	m, err := metricspkg.NewFromConfig(c, r.log)

	if err != nil {
		return nil, err
	}

	if c.Statsd.Enabled() {
		sw := metricspkg.NewStatsdWriter(c.Statsd, c.Tags, r.log)
		m.RegisterWriter(sw)
	}

	return m, nil
}

func (r *Runner) newController(metrics *metricspkg.Metrics) (node.Controller, error) {
	controller, err := r.controllerFactory(metrics, r.config, r.log)
	if err != nil {
		return nil, errorx.Decorate(err, "!!! Failed to initialize controller !!!")
	}

	ids := []identity.Identifier{}

	if r.config.JWT.Enabled() {
		ids = append(ids, identity.NewJWTIdentifier(&r.config.JWT, r.log))
		r.log.Info(fmt.Sprintf("JWT authentication is enabled (param: %s, enforced: %v)", r.config.JWT.Param, r.config.JWT.Force))
	}

	if r.config.SkipAuth {
		ids = append(ids, identity.NewPublicIdentifier())
		r.log.Info("connection authentication is disabled")
	}

	if len(ids) > 1 {
		identifier := identity.NewIdentifierPipeline(ids...)
		controller = identity.NewIdentifiableController(controller, identifier)
	} else if len(ids) == 1 {
		controller = identity.NewIdentifiableController(controller, ids[0])
	}

	if !r.Router().Empty() {
		r.Router().SetDefault(controller)
		controller = r.Router()
		r.log.Info(fmt.Sprintf("Using channels router: %s", strings.Join(r.Router().Routes(), ", ")))
	}

	return controller, nil
}

func (r *Runner) startWSServer(wsServer *server.HTTPServer) {
	go func() {
		err := wsServer.StartAndAnnounce("WebSocket server")
		if err != nil {
			if !wsServer.Stopped() {
				r.errChan <- fmt.Errorf("WebSocket server at %s stopped: %v", wsServer.Address(), err)
			}
		}
	}()
}

func (r *Runner) startMetrics(metrics *metricspkg.Metrics) {
	err := metrics.Run()
	if err != nil {
		r.errChan <- fmt.Errorf("!!! Metrics module failed to start !!!\n%v", err)
	}
}

func (r *Runner) defaultDisconnector(n *node.Node, c *config.Config, l *slog.Logger) (node.Disconnector, error) {
	if c.DisconnectorDisabled {
		return node.NewNoopDisconnector(), nil
	}
	return node.NewDisconnectQueue(n, &c.DisconnectQueue, l), nil
}

func (r *Runner) defaultWebSocketHandler(n *node.Node, c *config.Config, l *slog.Logger) (http.Handler, error) {
	extractor := server.DefaultHeadersExtractor{Headers: c.RPC.ProxyHeaders, Cookies: c.RPC.ProxyCookies}
	return ws.WebsocketHandler(common.ActionCableProtocols(), &extractor, &c.WS, r.log, func(wsc *websocket.Conn, info *server.RequestInfo, callback func()) error {
		wrappedConn := ws.NewConnection(wsc)

		opts := []node.SessionOption{}
		opts = append(opts, r.sessionOptionsFromProtocol(wsc.Subprotocol())...)
		opts = append(opts, r.sessionOptionsFromParams(info)...)

		session := node.NewSession(n, wrappedConn, info.URL, info.Headers, info.UID, opts...)

		if session.AuthenticateOnConnect() {
			_, err := n.Authenticate(session)

			if err != nil {
				return err
			}
		}

		return session.Serve(callback)
	}), nil
}

func (r *Runner) defaultSSEHandler(n *node.Node, ctx context.Context, c *config.Config) (http.Handler, error) {
	extractor := server.DefaultHeadersExtractor{Headers: c.RPC.ProxyHeaders, Cookies: c.RPC.ProxyCookies}
	handler := sse.SSEHandler(n, ctx, &extractor, &c.SSE, r.log)

	return handler, nil
}

func (r *Runner) initMRuby() string {
	if mrb.Supported() {
		var mrbv string
		mrbv, err := mrb.Version()
		if err != nil {
			r.log.Error(fmt.Sprintf("mruby failed to initialize: %v", err))
		} else {
			return " (with " + mrbv + ")"
		}
	}

	return ""
}

func (r *Runner) Router() *router.RouterController {
	if r.router == nil {
		r.SetRouter(r.defaultRouter())
	}

	return r.router
}

func (r *Runner) SetRouter(router *router.RouterController) {
	r.router = router
}

func (r *Runner) Instrumenter() metricspkg.Instrumenter {
	return r.metrics
}

func (r *Runner) defaultRouter() *router.RouterController {
	router := router.NewRouterController(nil)

	if r.config.Streams.PubSubChannel != "" {
		streamController := streams.NewStreamsController(&r.config.Streams, r.log)
		router.Route(r.config.Streams.PubSubChannel, streamController) // nolint:errcheck
	}

	if r.config.Streams.Turbo && r.config.Streams.GetTurboSecret() != "" {
		turboController := streams.NewTurboController(r.config.Streams.GetTurboSecret(), r.log)
		router.Route("Turbo::StreamsChannel", turboController) // nolint:errcheck
	}

	if r.config.Streams.CableReady && r.config.Streams.GetCableReadySecret() != "" {
		crController := streams.NewCableReadyController(r.config.Streams.GetCableReadySecret(), r.log)
		router.Route("CableReady::Stream", crController) // nolint:errcheck
	}

	return router
}

func (r *Runner) announceGoPools() {
	configs := make([]string, 0)
	pools := utils.AllPools()

	for _, pool := range pools {
		configs = append(configs, fmt.Sprintf("%s: %d", pool.Name(), pool.Size()))
	}

	r.log.Debug(fmt.Sprintf("Go pools initialized (%s)", strings.Join(configs, ", ")))
}

func (r *Runner) setupSignalHandlers() {
	s := utils.NewGracefulSignals(time.Duration(r.config.App.ShutdownTimeout) * time.Second)

	s.HandleForceTerminate(func() {
		r.log.Warn("Immediate termination requested. Stopped")
		r.errChan <- nil
	})

	s.Handle(func(ctx context.Context) error {
		r.log.Info(fmt.Sprintf("Shutting down... (hit Ctrl-C to stop immediately or wait for up to %ds for graceful shutdown)", r.config.App.ShutdownTimeout))
		return nil
	})

	for _, shutdownable := range r.shutdownables {
		s.Handle(shutdownable.Shutdown)
	}

	s.Handle(func(ctx context.Context) error {
		r.errChan <- nil
		return nil
	})

	s.Listen()
}

func (r *Runner) embedNATS(c *enats.Config) (*enats.Service, error) {
	service := enats.NewService(c, r.log)

	err := service.Start()

	if err != nil {
		return nil, err
	}

	return service, nil
}
