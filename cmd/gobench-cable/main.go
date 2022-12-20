package main

import (
	"fmt"
	"log"
	"os"

	"github.com/anycable/anycable-go/cli"
	"github.com/anycable/anycable-go/config"
	_ "github.com/anycable/anycable-go/diagnostics"
	"github.com/anycable/anycable-go/gobench"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/node"
)

func main() {
	c, err, ok := cli.NewConfigFromCLI(os.Args)
	if err != nil {
		log.Fatalf("%v", err)
	}
	if ok {
		os.Exit(0)
	}

	opts := []cli.Option{
		cli.WithName("GoBenchCable"),
		cli.WithController(func(m *metrics.Metrics, c *config.Config) (node.Controller, error) {
			return gobench.NewController(m), nil
		}),
		cli.WithDefaultBroker(),
		cli.WithDefaultSubscriber(),
		cli.WithDefaultBroadcaster(),
	}

	runner, err := cli.NewRunner(c, opts)

	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}

	err = runner.Run()

	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}
}
