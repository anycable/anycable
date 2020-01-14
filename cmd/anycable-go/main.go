package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/anycable/anycable-go/cli"
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

var (
	version string
)

func init() {
	if version == "" {
		version = "1.0.0-unknown"
	}
}

func main() {
	if cli.ShowVersion() {
		fmt.Println(version)
		os.Exit(0)
	}

	if cli.ShowHelp() {
		cli.PrintHelp()
		os.Exit(0)
	}

	config := cli.Config()

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
		mrbv, err := mrb.Version()
		if err != nil {
			ctx.Errorf("mruby failed to initialize: %v", err)
		} else {
			mrubySupport = " (with " + mrbv + ")"
		}
	}

	ctx.Infof("Starting AnyCable %s%s (pid: %d, open file limit: %s)", version, mrubySupport, os.Getpid(), utils.OpenFileLimit())

	metrics, err := metrics.FromConfig(&config.Metrics)

	if err != nil {
		log.Errorf("!!! Failed to initialize custom log printer !!!\n%v", err)
		os.Exit(1)
	}

	controller := rpc.NewController(metrics, &config.RPC)

	appNode := node.NewNode(controller, metrics)

	disconnector := node.NewDisconnectQueue(appNode, config.DisconnectRate)
	go disconnector.Run()

	appNode.SetDisconnector(disconnector)

	subscriber := pubsub.NewRedisSubscriber(appNode, config.RedisURL, config.RedisChannel)

	go func() {
		if err := subscriber.Start(); err != nil {
			ctx.Errorf("!!! Subscriber failed !!!\n%v", err)
			os.Exit(1)
		}
	}()

	go func() {
		if err := controller.Start(); err != nil {
			ctx.Errorf("!!! RPC failed !!!\n%v", err)
			os.Exit(1)
		}
	}()

	wsServer := buildServer(config.Host, strconv.Itoa(config.Port), &config.SSL)
	wsServer.Mux.Handle(config.Path, server.WebsocketHandler(appNode, config.Headers, &config.WS))

	ctx.Infof("Handle WebSocket connections at %s", config.Path)

	wsServer.Mux.Handle(config.HealthPath, http.HandlerFunc(wsServer.HealthHandler))
	ctx.Infof("Handle health connections at %s", config.HealthPath)

	go runServer(wsServer)

	go metrics.Run()

	t := tebata.New(syscall.SIGINT, syscall.SIGTERM)

	t.Reserve(func() {
		ctx.Infof("Shutting down... (hit Ctrl-C to stop immediately)")
		go func() {
			termSig := make(chan os.Signal, 1)
			signal.Notify(termSig, syscall.SIGINT, syscall.SIGTERM)
			<-termSig
			ctx.Warnf("Immediate termination requested. Stopped")
			os.Exit(0)
		}()
	})
	t.Reserve(metrics.Shutdown)
	t.Reserve(wsServer.Stop)
	t.Reserve(appNode.Shutdown)

	if config.Metrics.HTTPEnabled() {
		metricsServer := wsServer

		if config.Metrics.Port != config.Port {
			port := strconv.Itoa(config.Metrics.Port)
			host := config.Host
			if config.Metrics.Host != "" {
				host = config.Metrics.Host
			}
			metricsServer = buildServer(host, port, &config.SSL)
			ctx.Infof("Serve metrics at %s:%s%s", config.Host, port, config.Metrics.HTTP)
		} else {
			ctx.Infof("Serve metrics at %s", config.Metrics.HTTP)
		}

		metricsServer.Mux.Handle(config.Metrics.HTTP, http.HandlerFunc(metrics.PrometheusHandler))

		if metricsServer != wsServer {
			go runServer(metricsServer)
			t.Reserve(metricsServer.Stop)
		}
	}

	t.Reserve(os.Exit, 0)

	// Hang forever unless Exit is called
	select {}
}

func buildServer(host string, port string, ssl *server.SSLConfig) *server.HTTPServer {
	s, err := server.NewServer(host, port, ssl)

	if err != nil {
		fmt.Printf("!!! Failed to initialize HTTP server at %s:%s !!!\n%v", err, host, port)
		os.Exit(1)
	}

	return s
}

func runServer(s *server.HTTPServer) {
	if err := s.Start(); err != nil {
		if !s.Stopped() {
			log.Errorf("HTTP server at %s stopped: %v", err, s.Address())
			os.Exit(1)
		}
	}
}
