package cli

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/anycable/anycable-go/config"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mrb"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/utils"
	"github.com/anycable/anycable-go/version"
	"github.com/anycable/anycable-go/ws"
	"github.com/apex/log"
	"github.com/gorilla/websocket"
	"github.com/syossan27/tebata"
)

type controllerFactory = func(*metrics.Metrics, *config.Config) (node.Controller, error)
type disconnectorFactory = func(*node.Node, *config.Config) (node.Disconnector, error)
type subscriberFactory = func(*node.Node, *config.Config) (pubsub.Subscriber, error)
type websocketHandler = func(*node.Node, *config.Config) (http.Handler, error)

type Shutdownable interface {
	Shutdown() error
}

type Runner struct {
	name                string
	config              *config.Config
	controllerFactory   controllerFactory
	disconnectorFactory disconnectorFactory
	subscriberFactory   subscriberFactory
	websocketHandler    websocketHandler

	errChan       chan error
	shutdownables []Shutdownable
}

func NewRunner(name string, config *config.Config) *Runner {
	if name == "" {
		name = "AnyCable"
	}

	if config == nil {
		config = Config()
	}

	// Set global HTTP params as early as possible to make sure all servers use them
	server.SSL = &config.SSL
	server.Host = config.Host

	return &Runner{name: name, config: config, shutdownables: []Shutdownable{}, errChan: make(chan error)}
}

func (r *Runner) ControllerFactory(fn controllerFactory) {
	r.controllerFactory = fn
}

func (r *Runner) DisconnectorFactory(fn disconnectorFactory) {
	r.disconnectorFactory = fn
}

func (r *Runner) SubscriberFactory(fn subscriberFactory) {
	r.subscriberFactory = fn
}

func (r *Runner) WebsocketHandler(fn websocketHandler) {
	r.websocketHandler = fn
}

func (r *Runner) Run() error {
	if ShowVersion() {
		fmt.Println(version.Version())
		return nil
	}

	if ShowHelp() {
		PrintHelp()
		return nil
	}

	config := r.config

	// init logging
	err := utils.InitLogger(config.LogFormat, config.LogLevel)

	if err != nil {
		return fmt.Errorf("!!! Failed to initialize logger !!!\n%v", err)
	}

	ctx := log.WithFields(log.Fields{"context": "main"})

	if DebugMode() {
		ctx.Debug("🔧 🔧 🔧 Debug mode is on 🔧 🔧 🔧")
	}

	mrubySupport := r.initMRuby()

	ctx.Infof("Starting %s %s%s (pid: %d, open file limit: %s)", r.name, version.Version(), mrubySupport, os.Getpid(), utils.OpenFileLimit())

	metrics, err := r.initMetrics(&config.Metrics)

	if err != nil {
		return fmt.Errorf("!!! Failed to initialize metrics writer !!!\n%v", err)
	}

	r.shutdownables = append(r.shutdownables, metrics)

	controller, err := r.initController(metrics, config)

	if err != nil {
		return fmt.Errorf("!!! Failed to initialize controller !!!\n%v", err)
	}

	appNode := node.NewNode(controller, metrics, &config.App)
	err = appNode.Start()

	if err != nil {
		return fmt.Errorf("!!! Failed to initialize application !!!\n%v", err)
	}

	disconnector, err := r.initDisconnector(appNode, config)

	if err != nil {
		return fmt.Errorf("!!! Failed to initialize disconnector !!!\n%v", err)
	}

	go disconnector.Run() // nolint:errcheck
	appNode.SetDisconnector(disconnector)

	subscriber, err := r.initSubscriber(appNode, config)

	if err != nil {
		return fmt.Errorf("Couldn't configure pub/sub: %v", err)
	}

	r.shutdownables = append(r.shutdownables, subscriber)

	go func() {
		if subscribeErr := subscriber.Start(); subscribeErr != nil {
			r.errChan <- fmt.Errorf("!!! Subscriber failed !!!\n%v", subscribeErr)
		}
	}()

	go func() {
		if contrErr := controller.Start(); contrErr != nil {
			r.errChan <- fmt.Errorf("!!! RPC failed !!!\n%v", contrErr)
		}
	}()

	wsServer, err := server.ForPort(strconv.Itoa(config.Port))
	if err != nil {
		return fmt.Errorf("!!! Failed to initialize WebSocket server at %s:%s !!!\n%v", err, config.Host, config.Port)
	}

	r.shutdownables = append(r.shutdownables, wsServer)

	wsHandler, err := r.initWebSocketHandler(appNode, config)
	if err != nil {
		return fmt.Errorf("!!! Failed to initialize WebSocket handler !!!\n%v", err)
	}

	wsServer.Mux.Handle(config.Path, wsHandler)

	ctx.Infof("Handle WebSocket connections at %s%s", wsServer.Address(), config.Path)

	wsServer.Mux.Handle(config.HealthPath, http.HandlerFunc(server.HealthHandler))
	ctx.Infof("Handle health connections at %s%s", wsServer.Address(), config.HealthPath)

	go func() {
		if err = wsServer.StartAndAnnounce("WebSocket server"); err != nil {
			if !wsServer.Stopped() {
				r.errChan <- fmt.Errorf("WebSocket server at %s stopped: %v", wsServer.Address(), err)
			}
		}
	}()

	go func() {
		if err := metrics.Run(); err != nil {
			r.errChan <- fmt.Errorf("!!! Metrics module failed to start !!!\n%v", err)
		}
	}()

	r.shutdownables = append(r.shutdownables, appNode)

	r.setupSignalHandlers()

	// Wait for an error (or none)
	return <-r.errChan
}

func (r *Runner) initMetrics(c *metrics.Config) (*metrics.Metrics, error) {
	m, err := metrics.FromConfig(c)

	if err != nil {
		return nil, err
	}

	return m, nil
}

func (r *Runner) initController(m *metrics.Metrics, c *config.Config) (node.Controller, error) {
	if r.controllerFactory == nil {
		return nil, errors.New("Controller factory is not specified")
	}

	return r.controllerFactory(m, c)
}

func (r *Runner) initDisconnector(n *node.Node, c *config.Config) (node.Disconnector, error) {
	if r.disconnectorFactory == nil {
		return r.defaultDisconnector(n, c)
	}

	return r.disconnectorFactory(n, c)
}

func (r *Runner) defaultDisconnector(n *node.Node, c *config.Config) (node.Disconnector, error) {
	if c.DisconnectorDisabled {
		return node.NewNoopDisconnector(), nil
	} else {
		return node.NewDisconnectQueue(n, &c.DisconnectQueue), nil
	}
}

func (r *Runner) initSubscriber(n *node.Node, c *config.Config) (pubsub.Subscriber, error) {
	if r.subscriberFactory == nil {
		return nil, errors.New("Subscriber factory is not specified")
	}

	return r.subscriberFactory(n, c)
}

func (r *Runner) initWebSocketHandler(n *node.Node, c *config.Config) (http.Handler, error) {
	if r.websocketHandler == nil {
		return r.defaultWebSocketHandler(n, c), nil
	}

	return r.websocketHandler(n, c)
}

func (r *Runner) defaultWebSocketHandler(n *node.Node, c *config.Config) http.Handler {
	return ws.WebsocketHandler(c.Headers, &c.WS, func(wsc *websocket.Conn, info *ws.RequestInfo) error {
		wrappedConn := ws.NewConnection(wsc)
		session := node.NewSession(n, wrappedConn, info.Url, info.Headers, info.UID)

		_, err := n.Authenticate(session)

		if err != nil {
			return err
		}

		session.Serve()
		return nil
	})
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
