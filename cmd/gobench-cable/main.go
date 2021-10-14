package main

import (
	"fmt"
	"os"

	"github.com/anycable/anycable-go/cli"
	"github.com/anycable/anycable-go/config"
	_ "github.com/anycable/anycable-go/diagnostics"
	"github.com/anycable/anycable-go/gobench"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/pubsub"
)

func main() {
	// Default runner
	runner := cli.NewRunner("GoBenchCable", nil)

	runner.ControllerFactory(func(m *metrics.Metrics, c *config.Config) (node.Controller, error) {
		return gobench.NewController(m), nil
	})

	runner.SubscriberFactory(func(h pubsub.Handler, c *config.Config) (pubsub.Subscriber, error) {
		return pubsub.NewSubscriber(h, c.BroadcastAdapter, &c.Redis, &c.HTTPPubSub)
	})

	err := runner.Run()

	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
