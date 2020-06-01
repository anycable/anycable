package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/anycable/anycable-go/cli"
	"github.com/anycable/anycable-go/gobench"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/node"
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

	ctx.Infof("Starting GoBenchCable %s (pid: %d, open file limit: %s)", utils.Version(), os.Getpid(), utils.OpenFileLimit())

	metrics, err := metrics.FromConfig(&config.Metrics)

	if err != nil {
		log.Errorf("!!! Failed to initialize custom log printer !!!\n%v", err)
		os.Exit(1)
	}

	controller := gobench.NewController(metrics)

	appNode := node.NewNode(controller, metrics)

	// There could be different disconnectors in the future
	disconnector := node.NewDisconnectQueue(appNode, &config.DisconnectQueue)
	go disconnector.Run()

	appNode.SetDisconnector(disconnector)

	go func() {
		if err := controller.Start(); err != nil {
			ctx.Errorf("!!! Controller failed !!!\n%v", err)
			os.Exit(1)
		}
	}()

	server.SSL = &config.SSL
	server.Host = config.Host

	wsServer, err := server.ForPort(strconv.Itoa(config.Port))
	if err != nil {
		fmt.Printf("!!! Failed to initialize WebSocket server at %s:%s !!!\n%v", err, config.Host, config.Port)
		os.Exit(1)
	}

	wsServer.Mux.Handle(config.Path, node.WebsocketHandler(appNode, config.Headers, &config.WS))
	ctx.Infof("Handle WebSocket connections at %s", config.Path)

	wsServer.Mux.Handle(config.HealthPath, http.HandlerFunc(server.HealthHandler))
	ctx.Infof("Handle health connections at %s", config.HealthPath)

	go func() {
		if err = wsServer.Start(); err != nil {
			if !wsServer.Stopped() {
				log.Errorf("WebSocket server at %s stopped: %v", err, wsServer.Address())
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

	t.Reserve(os.Exit, 0)

	// Hang forever unless Exit is called
	select {}
}
