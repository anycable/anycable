package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"syscall"

	"github.com/anycable/anycable-go/cli"
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
		version = "0.6.0-unknown"
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

	ctx := log.WithFields(log.Fields{"context": "main"})

	// init logging
	err := utils.InitLogger(config.LogFormat, config.LogLevel)

	if err != nil {
		log.Errorf("!!! Failed to initialize logger !!!\n%v", err)
		os.Exit(1)
	}

	if cli.DebugMode() {
		ctx.Debug("🔧 🔧 🔧 Debug mode is on 🔧 🔧 🔧")
	}

	ctx.Infof("Starting AnyCable %s", version)

	controller := rpc.NewController(&config)

	node := node.NewNode(&config, controller)

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

	// TODO: init metrics

	server, err := server.NewServer(node, config.Host, strconv.Itoa(config.Port), &config.SSL)

	if err != nil {
		fmt.Printf("!!! Failed to initialize HTTP server !!!\n%v", err)
		os.Exit(1)
	}

	t := tebata.New(syscall.SIGINT, syscall.SIGTERM)

	done := make(chan bool)

	t.Reserve(server.Stop)
	t.Reserve(node.Shutdown)

	server.Mux.Handle(config.Path, http.HandlerFunc(server.WebsocketHandler))

	ctx.Infof("Handle WebSocket connections at %s", config.Path)

	t.Reserve(os.Exit, 0)

	if err = server.Start(); err != nil {
		if !server.Stopped() {
			ctx.Errorf("Server stopped: %v", err)
		}
	}

	<-done
}
