package main

import (
	"fmt"
	"os"

	"github.com/anycable/anycable-go/cli"
	"github.com/anycable/anycable-go/config"
	_ "github.com/anycable/anycable-go/diagnostics"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/anycable/anycable-go/rpc"
)

func main() {
	// Default runner
	runner := cli.NewRunner("", nil)

	runner.ControllerFactory(func(m *metrics.Metrics, c *config.Config) (node.Controller, error) {
		return rpc.NewController(m, &c.RPC), nil
	})

	runner.SubscriberFactory(func(n *node.Node, c *config.Config) (pubsub.Subscriber, error) {
		return pubsub.NewSubscriber(n, c.BroadcastAdapter, &c.Redis, &c.HTTPPubSub)
	})

	err := runner.Run()

	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
