package main

import (
	"fmt"
	"log"
	"os"

	"github.com/anycable/anycable-go/cli"
	_ "github.com/anycable/anycable-go/diagnostics"

	_ "unsafe"
)

// IgnorePC is responsible for adding callers pointer to log records.
// We don't use `AddSource` in our handler, so why not dropping the `runtime.Callers` overhead?
// See also https://github.com/rs/zerolog/issues/571#issuecomment-1697479194
//
//go:linkname IgnorePC log/slog/internal.IgnorePC
var IgnorePC = true

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
		cli.WithDefaultBroker(),
		cli.WithDefaultSubscriber(),
		cli.WithDefaultBroadcaster(),
		cli.WithTelemetry(),
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
