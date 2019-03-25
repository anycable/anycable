package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/anycable/anycable-go/cli"
	"github.com/anycable/anycable-go/config"
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
		version = "0.6.2-unknown"
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

	config := cli.GetConfig()

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
			log.Errorf("mruby failed to initialize: %v", err)
		} else {
			mrubySupport = " (with " + mrbv + ")"
		}
	}

	ctx.Infof("Starting AnyCable %s%s (pid: %d)", version, mrubySupport, os.Getpid())

	var metricsPrinter metrics.Printer

	if config.MetricsLogEnabled() {
		if config.MetricsLogFormatterEnabled() {
			customPrinter, err := metrics.NewCustomPrinter(config.MetricsLogFormatter)

			if err == nil {
				metricsPrinter = customPrinter
			} else {
				log.Errorf("!!! Failed to initialize custom log printer !!!\n%v", err)
				os.Exit(1)
			}
		} else {
			metricsPrinter = metrics.NewBasePrinter()
		}
	}

	metrics := metrics.NewMetrics(metricsPrinter, config.MetricsLogInterval)

	controller := rpc.NewController(&config, metrics)

	node := node.NewNode(&config, controller, metrics)

	subscriber := pubsub.NewRedisSubscriber(node, config.RedisURL, config.RedisChannel)

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

	wsServer := buildServer(node, config.Host, strconv.Itoa(config.Port), &config.SSL)
	wsServer.Mux.Handle(config.Path, http.HandlerFunc(wsServer.WebsocketHandler))
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
	t.Reserve(node.Shutdown)

	if config.MetricsHTTPEnabled() {
		metricsServer := wsServer

		if config.MetricsPort != config.Port {
			port := strconv.Itoa(config.MetricsPort)
			host := config.Host
			if config.MetricsHost != "" {
				host = config.MetricsHost
			}
			metricsServer = buildServer(node, host, port, &config.SSL)
			ctx.Infof("Serve metrics at %s:%s%s", config.Host, port, config.MetricsHTTP)
		} else {
			ctx.Infof("Serve metrics at %s", config.MetricsHTTP)
		}

		metricsServer.Mux.Handle(config.MetricsHTTP, http.HandlerFunc(metrics.PrometheusHandler))

		if metricsServer != wsServer {
			go runServer(metricsServer)
			t.Reserve(metricsServer.Stop)
		}
	}

	t.Reserve(os.Exit, 0)

	// Hang forever unless Exit is called
	select {}
}

func buildServer(node *node.Node, host string, port string, ssl *config.SSLOptions) *server.HTTPServer {
	s, err := server.NewServer(node, host, port, ssl)

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
