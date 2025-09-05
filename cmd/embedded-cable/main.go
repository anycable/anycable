package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/anycable/anycable-go/cli"
	_ "github.com/anycable/anycable-go/diagnostics"
	"github.com/anycable/anycable-go/utils"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))

	opts := []cli.Option{
		cli.WithName("AnyCable"),
		cli.WithDefaultRPCController(),
		cli.WithDefaultBroker(),
		cli.WithDefaultSubscriber(),
		cli.WithTelemetry(),
		cli.WithLogger(logger),
	}

	c := cli.NewConfig()

	runner, err := cli.NewRunner(c, opts)

	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}

	anycable, err := runner.Embed()

	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}

	wsHandler, err := anycable.WebSocketHandler()

	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}

	seeHandler, err := anycable.SSEHandler(context.Background())

	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}

	broadcastHandler, err := anycable.HTTPBroadcastHandler()

	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}

	http.Handle("/cable", wsHandler)
	http.Handle("/sse", seeHandler)
	http.Handle("/broadcast", broadcastHandler)

	go http.ListenAndServe(":8080", nil) // nolint:errcheck,gosec

	// Graceful shutdown (to ensure AnyCable sends disconnect notices)
	s := utils.NewGracefulSignals(10 * time.Second)
	ch := make(chan error, 1)

	s.HandleForceTerminate(func() {
		logger.Warn("Immediate termination requested. Stopped")
		ch <- nil
	})

	s.Handle(func(ctx context.Context) error {
		logger.Info("Shutting down... (hit Ctrl-C to stop immediately or wait for up to 10s for graceful shutdown)")
		return nil
	})
	s.Handle(anycable.Shutdown)
	s.Handle(func(ctx context.Context) error {
		ch <- nil
		return nil
	})

	s.Listen()

	<-ch
}
