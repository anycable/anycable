package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/anycable/anycable-go/cli"
	_ "github.com/anycable/anycable-go/diagnostics"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mrb"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/anycable/anycable-go/rpc"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/utils"

	log "github.com/apex/log"
	"github.com/syossan27/tebata"
)

func main() {
	if cli.ShowVersion() {
		fmt.Println(utils.Version())
		os.Exit(0)
	}

	if cli.ShowHelp() {
		cli.PrintHelp()
		os.Exit(0)
	}

	config := cli.Config()

	// Set global HTTP params as early as possible to make sure all servers use them
	server.SSL = &config.SSL
	server.Host = config.Host

	// init logging
	err := utils.InitLogger(config.LogFormat, config.LogLevel)

	if err != nil {
		log.Errorf("!!! Failed to initialize logger !!!\n%v", err)
		os.Exit(1)
	}

	ctx := log.WithFields(log.Fields{"context": "main"})

	if cli.DebugMode() {
		ctx.Debug("🔧 🔧 🔧 Debug mode is on 🔧 🔧 🔧")
	}

	mrubySupport := ""

	if mrb.Supported() {
		var mrbv string
		mrbv, err = mrb.Version()
		if err != nil {
			ctx.Errorf("mruby failed to initialize: %v", err)
		} else {
			mrubySupport = " (with " + mrbv + ")"
		}
	}

	ctx.Infof("Starting AnyCable %s%s (pid: %d, open file limit: %s)", utils.Version(), mrubySupport, os.Getpid(), utils.OpenFileLimit())

	metrics, err := metrics.FromConfig(&config.Metrics)

	if err != nil {
		log.Errorf("!!! Failed to initialize custom log printer !!!\n%v", err)
		os.Exit(1)
	}

	controller := rpc.NewController(metrics, &config.RPC)

	appNode := node.NewNode(controller, metrics, &config.App)
	err = appNode.Start()

	if err != nil {
		log.Errorf("!!! Failed to initialize application !!!\n%v", err)
		os.Exit(1)
	}

	var disconnector node.Disconnector

	if config.DisconnectorDisabled {
		disconnector = node.NewNoopDisconnector()
	} else {
		disconnector = node.NewDisconnectQueue(appNode, &config.DisconnectQueue)
	}

	go disconnector.Run() // nolint:errcheck
	appNode.SetDisconnector(disconnector)

	subscriber, err := pubsub.NewSubscriber(appNode, config.BroadcastAdapter, &config.Redis, &config.HTTPPubSub)

	if err != nil {
		ctx.Errorf("Couldn't configure pub/sub: %v", err)
		os.Exit(1)
	}

	go func() {
		if subscribeErr := subscriber.Start(); subscribeErr != nil {
			ctx.Errorf("!!! Subscriber failed !!!\n%v", subscribeErr)
			os.Exit(1)
		}
	}()

	go func() {
		if contrErr := controller.Start(); contrErr != nil {
			ctx.Errorf("!!! RPC failed !!!\n%v", contrErr)
			os.Exit(1)
		}
	}()

	wsServer, err := server.ForPort(strconv.Itoa(config.Port))
	if err != nil {
		fmt.Printf("!!! Failed to initialize WebSocket server at %s:%s !!!\n%v", err, config.Host, config.Port)
		os.Exit(1)
	}

	wsServer.Mux.Handle(config.Path, node.WebsocketHandler(appNode, config.Headers, &config.WS))

	ctx.Infof("Handle WebSocket connections at %s%s", wsServer.Address(), config.Path)

	wsServer.Mux.Handle(config.HealthPath, http.HandlerFunc(server.HealthHandler))
	ctx.Infof("Handle health connections at %s%s", wsServer.Address(), config.HealthPath)

	go func() {
		if err = wsServer.StartAndAnnounce("WebSocket server"); err != nil {
			if !wsServer.Stopped() {
				log.Errorf("WebSocket server at %s stopped: %v", wsServer.Address(), err)
				os.Exit(1)
			}
		}
	}()

	go func() {
		if err := metrics.Run(); err != nil {
			ctx.Errorf("!!! Metrics module failed to start !!!\n%v", err)
			os.Exit(1)
		}
	}()

	t := tebata.New(syscall.SIGINT, syscall.SIGTERM)

	t.Reserve(func() { // nolint:errcheck
		ctx.Infof("Shutting down... (hit Ctrl-C to stop immediately)")
		go func() {
			termSig := make(chan os.Signal, 1)
			signal.Notify(termSig, syscall.SIGINT, syscall.SIGTERM)
			<-termSig
			ctx.Warnf("Immediate termination requested. Stopped")
			os.Exit(0)
		}()
	})
	t.Reserve(metrics.Shutdown)    // nolint:errcheck
	t.Reserve(subscriber.Shutdown) // nolint:errcheck
	t.Reserve(wsServer.Stop)       // nolint:errcheck
	t.Reserve(appNode.Shutdown)    // nolint:errcheck

	t.Reserve(os.Exit, 0) // nolint:errcheck

	// Hang forever unless Exit is called
	select {}
}
