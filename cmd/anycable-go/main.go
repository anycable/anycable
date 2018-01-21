package main

import (
	"fmt"
	"os"

	"github.com/anycable/anycable-go/cli"
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

	// init application (RPC + pubsub listener + hub + metrics)

	// init signals handlers

	// init server

	// run server
}
