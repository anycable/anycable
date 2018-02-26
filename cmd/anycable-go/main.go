package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/anycable/anycable-go/cli"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/utils"
	log "github.com/apex/log"
)

var (
	version string
)

func init() {
	if version == "" {
		version = "unknown"
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

	fmt.Println(config)

	// init logging
	err := utils.InitLogger(config.LogFormat, config.LogLevel)

	if err != nil {
		fmt.Printf("!!! Failed to initialize logger !!!\n%v", err)
		os.Exit(1)
	}

	if cli.DebugMode() {
		log.Debug("🔧 🔧 🔧 Debug mode is on 🔧 🔧 🔧")
	}

	log.Infof("Starting AnyCable %s", version)

	node := node.NewNode(&config)

	subscriber := pubsub.NewRedisSubscriber(node, config.RedisURL, config.RedisChannel)

	go func() {
		if err := subscriber.Start(); err != nil {
			os.Exit(1)
		}
	}()

	// init application (RPC + metrics)

	// init signals handlers

	// init server
	server, err := server.NewServer(node, config.Host, strconv.Itoa(config.Port), &config.SSL)

	if err != nil {
		fmt.Printf("!!! Failed to initialize HTTP server !!!\n%v", err)
		os.Exit(1)
	}

	server.Mux.Handle(config.Path, http.HandlerFunc(server.WebsocketHandler))

	log.Infof("Handle WebSocket connections at %s", config.Path)

	server.Start()
}
