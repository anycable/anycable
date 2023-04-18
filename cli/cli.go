package cli

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/anycable/anycable-go/broadcast"
	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/config"
	"github.com/anycable/anycable-go/enats"
	"github.com/anycable/anycable-go/identity"
	metricspkg "github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mrb"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/anycable/anycable-go/rails"
	"github.com/anycable/anycable-go/router"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/utils"
	"github.com/anycable/anycable-go/version"
	"github.com/anycable/anycable-go/ws"
	"github.com/apex/log"
	"github.com/gorilla/websocket"
	"github.com/joomcode/errorx"
	"github.com/syossan27/tebata"

	"go.uber.org/automaxprocs/maxprocs"
)

type controllerFactory = func(*metricspkg.Metrics, *config.Config) (node.Controller, error)
type disconnectorFactory = func(*node.Node, *config.Config) (node.Disconnector, error)
type broadcasterFactory = func(broadcast.Handler, *config.Config) (broadcast.Broadcaster, error)
type brokerFactory = func(broker.Broadcaster, *config.Config) (broker.Broker, error)
type subscriberFactory = func(pubsub.Handler, *config.Config) (pubsub.Subscriber, error)
type websocketHandler = func(*node.Node, *config.Config) (http.Handler, error)

type Shutdownable interface {
	Shutdown() error
}

type Runner struct {
	options []Option

	name   string
	config *config.Config
	log    *log.Entry

	controllerFactory       controllerFactory
	disconnectorFactory     disconnectorFactory
	subscriberFactory       subscriberFactory
	brokerFactory           brokerFactory
	websocketHandlerFactory websocketHandler

	broadcasters       []broadcasterFactory
	websocketEndpoints map[string]websocketHandler

	router *router.RouterController

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

	err := utils.InitLogger(r.config.LogFormat, r.config.LogLevel)
	if err != nil {
		return errorx.Decorate(err, "!!! Failed to initialize logger !!!")
	}

	r.log = log.WithFields(log.Fields{"context": "main"})

	err = r.config.LoadPresets()

	if err != nil {
		return errorx.Decorate(err, "!!! Failed to load configuration presets !!!")
	}

	server.SSL = &r.config.SSL
	server.Host = r.config.Host
	server.MaxConn = r.config.MaxConn

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

	return nil
}

// Run starts the instance
func (r *Runner) Run() error {
	numProcs := r.setMaxProcs()
	r.announceDebugMode()

	mrubySupport := r.initMRuby()

	r.log.Infof("Starting %s %s%s (pid: %d, open file limit: %s, gomaxprocs: %d)", r.name, version.Version(), mrubySupport, os.Getpid(), utils.OpenFileLimit(), numProcs)

	metrics, err := r.initMetrics(&r.config.Metrics)
	if err != nil {
		return errorx.Decorate(err, "!!! Failed to initialize metrics writer !!!")
	}

	r.shutdownables = append(r.shutdownables, metrics)

	controller, err := r.newController(metrics)
	if err != nil {
		return err
	}

	appNode := node.NewNode(controller, metrics, &r.config.App)

	subscriber, err := r.subscriberFactory(appNode, r.config)

	if err != nil {
		return errorx.Decorate(err, "couldn't configure pub/sub")
	}

	appBroker, err := r.brokerFactory(subscriber, r.config)
	if err != nil {
		return errorx.Decorate(err, "!!! Failed to initialize broker !!!")
	}

	if appBroker != nil {
		r.log.Infof(appBroker.Announce())
		appNode.SetBroker(appBroker)
	}

	disconnector, err := r.disconnectorFactory(appNode, r.config)
	if err != nil {
		return errorx.Decorate(err, "!!! Failed to initialize disconnector !!!")
	}

	go disconnector.Run() // nolint:errcheck
	appNode.SetDisconnector(disconnector)

	err = appNode.Start()

	if err != nil {
		return errorx.Decorate(err, "!!! Failed to initialize application !!!")
	}

	if r.config.EmbedNats {
		service, enatsErr := r.embedNATS(&r.config.EmbeddedNats)

		if enatsErr != nil {
			return errorx.Decorate(enatsErr, "failed to start embedded NATS server")
		}

		desc := service.Description()

		if desc != "" {
			desc = fmt.Sprintf(" (%s)", desc)
		}

		r.log.Infof("Embedded NATS server started: %s%s", r.config.EmbeddedNats.ServiceAddr, desc)

		r.shutdownables = append(r.shutdownables, service)
	}

	err = subscriber.Start(r.errChan)
	if err != nil {
		return errorx.Decorate(err, "!!! Subscriber failed !!!")
	}

	r.shutdownables = append(r.shutdownables, subscriber)

	if r.broadcasters != nil {
		for _, broadcasterFactory := range r.broadcasters {
			broadcaster, berr := broadcasterFactory(appNode, r.config)

			if berr != nil {
				return errorx.Decorate(err, "couldn't configure broadcaster")
			}

			if broadcaster.IsFanout() && subscriber.IsMultiNode() {
				r.log.Warnf("Using pub/sub with a distributed broadcaster has no effect")
			}

			if !broadcaster.IsFanout() && !subscriber.IsMultiNode() {
				r.log.Warnf("Using a non-distributed broadcaster without a pub/sub enabled; each broadcasted message is only processed by a single node")
			}

			err = broadcaster.Start(r.errChan)
			if err != nil {
				return errorx.Decorate(err, "!!! Broadcaster failed !!!")
			}

			r.shutdownables = append(r.shutdownables, broadcaster)
		}
	}

	err = controller.Start()
	if err != nil {
		return errorx.Decorate(err, "!!! RPC failed !!!")
	}

	wsServer, err := server.ForPort(strconv.Itoa(r.config.Port))
	if err != nil {
		return errorx.Decorate(err, "!!! Failed to initialize WebSocket server at %s:%d !!!", r.config.Host, r.config.Port)
	}

	wsHandler, err := r.websocketHandlerFactory(appNode, r.config)
	if err != nil {
		return errorx.Decorate(err, "!!! Failed to initialize WebSocket handler !!!")
	}

	for _, path := range r.config.Path {
		wsServer.SetupHandler(path, wsHandler)
		r.log.Infof("Handle WebSocket connections at %s%s", wsServer.Address(), path)
	}

	for path, handlerFactory := range r.websocketEndpoints {
		handler, err := handlerFactory(appNode, r.config)
		if err != nil {
			return errorx.Decorate(err, "!!! Failed to initialize WebSocket handler for %s !!!", path)
		}
		wsServer.SetupHandler(path, handler)
	}

	wsServer.SetupHandler(r.config.HealthPath, http.HandlerFunc(server.HealthHandler))
	r.log.Infof("Handle health requests at %s%s", wsServer.Address(), r.config.HealthPath)

	go r.startWSServer(wsServer)
	go r.startMetrics(metrics)

	r.shutdownables = append(r.shutdownables, appBroker, appNode)

	r.announceGoPools()
	r.setupSignalHandlers()

	// Wait for an error (or none)
	return <-r.errChan
}

func (r *Runner) setMaxProcs() int {
	// See https://github.com/uber-go/automaxprocs/issues/18
	nopLog := func(string, ...interface{}) {}
	maxprocs.Set(maxprocs.Logger(nopLog)) // nolint:errcheck

	return runtime.GOMAXPROCS(0)
}

func (r *Runner) announceDebugMode() {
	if r.config.Debug {
		r.log.Debug("🔧 🔧 🔧 Debug mode is on 🔧 🔧 🔧")
	}
}

func (r *Runner) initMetrics(c *metricspkg.Config) (*metricspkg.Metrics, error) {
	m, err := metricspkg.NewFromConfig(c)

	if err != nil {
		return nil, err
	}

	if c.Statsd.Enabled() {
		sw := metricspkg.NewStatsdWriter(c.Statsd, c.Tags)
		m.RegisterWriter(sw)
	}

	return m, nil
}

func (r *Runner) newController(metrics *metricspkg.Metrics) (node.Controller, error) {
	controller, err := r.controllerFactory(metrics, r.config)
	if err != nil {
		return nil, errorx.Decorate(err, "!!! Failed to initialize controller !!!")
	}

	if r.config.JWT.Enabled() {
		identifier := identity.NewJWTIdentifier(&r.config.JWT)
		controller = identity.NewIdentifiableController(controller, identifier)
		r.log.Infof("JWT identification is enabled (param: %s, enforced: %v)", r.config.JWT.Param, r.config.JWT.Force)
	}

	if !r.Router().Empty() {
		r.Router().SetDefault(controller)
		controller = r.Router()
		r.log.Infof("Using channels router: %s", strings.Join(r.Router().Routes(), ", "))
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

func (r *Runner) defaultDisconnector(n *node.Node, c *config.Config) (node.Disconnector, error) {
	if c.DisconnectorDisabled {
		return node.NewNoopDisconnector(), nil
	}
	return node.NewDisconnectQueue(n, &c.DisconnectQueue), nil
}

func (r *Runner) defaultWebSocketHandler(n *node.Node, c *config.Config) (http.Handler, error) {
	extractor := ws.DefaultHeadersExtractor{Headers: c.Headers, Cookies: c.Cookies}
	return ws.WebsocketHandler(common.ActionCableProtocols(), &extractor, &c.WS, func(wsc *websocket.Conn, info *ws.RequestInfo, callback func()) error {
		wrappedConn := ws.NewConnection(wsc)
		session := node.NewSession(n, wrappedConn, info.URL, info.Headers, info.UID)

		_, err := n.Authenticate(session)

		if err != nil {
			return err
		}

		return session.Serve(callback)
	}), nil
}

func (r *Runner) initMRuby() string {
	if mrb.Supported() {
		var mrbv string
		mrbv, err := mrb.Version()
		if err != nil {
			log.Errorf("mruby failed to initialize: %v", err)
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

func (r *Runner) defaultRouter() *router.RouterController {
	router := router.NewRouterController(nil)

	if r.config.Rails.TurboRailsKey != "" {
		turboController := rails.NewTurboController(r.config.Rails.TurboRailsKey)
		router.Route("Turbo::StreamsChannel", turboController) // nolint:errcheck
	}

	if r.config.Rails.CableReadyKey != "" {
		crController := rails.NewCableReadyController(r.config.Rails.CableReadyKey)
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

	log.WithField("context", "main").Debugf("Go pools initialized (%s)", strings.Join(configs, ", "))
}

func (r *Runner) setupSignalHandlers() {
	t := tebata.New(syscall.SIGINT, syscall.SIGTERM)

	t.Reserve(func() { // nolint:errcheck
		log.Infof("Shutting down... (hit Ctrl-C to stop immediately)")
		go func() {
			termSig := make(chan os.Signal, 1)
			signal.Notify(termSig, syscall.SIGINT, syscall.SIGTERM)
			<-termSig
			log.Warnf("Immediate termination requested. Stopped")
			r.errChan <- nil
		}()
	})

	for _, shutdownable := range r.shutdownables {
		t.Reserve(shutdownable.Shutdown) // nolint:errcheck
	}

	t.Reserve(func() { r.errChan <- nil }) // nolint:errcheck
}

func (r *Runner) embedNATS(c *enats.Config) (*enats.Service, error) {
	service := enats.NewService(c)

	err := service.Start()

	if err != nil {
		return nil, err
	}

	return service, nil
}
