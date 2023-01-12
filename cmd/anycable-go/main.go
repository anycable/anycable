package main

import (
	"fmt"
	"log"
	"os"

	"github.com/anycable/anycable-go/cli"
	_ "github.com/anycable/anycable-go/diagnostics"
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
		cli.WithName("AnyCable"),
		cli.WithDefaultRPCController(),
		cli.WithDefaultSubscriber(),
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
