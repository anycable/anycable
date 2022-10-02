package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/anycable/anycable-go/config"
	"github.com/anycable/anycable-go/version"
	"github.com/urfave/cli/v2"
)

// NewConfigFromCLI reads config from os.Args. It returns config, error (if any) and a bool value
// indicating that the usage message or version was shown, no further action required.
func NewConfigFromCLI() (*config.Config, error, bool) {
	c := config.NewConfig()

	var path, headers string
	var helpOrVersionWereShown bool = true

	// Print raw version without prefix
	cli.VersionPrinter = func(cCtx *cli.Context) {
		_, _ = fmt.Fprintf(cCtx.App.Writer, "%v\n", cCtx.App.Version)
	}

	flags := []cli.Flag{}
	flags = append(flags, serverCLIFlags(&c, &path)...)
	flags = append(flags, sslCLIFlags(&c)...)
	flags = append(flags, broadcastCLIFlags(&c)...)
	flags = append(flags, redisCLIFlags(&c)...)
	flags = append(flags, httpBroadcastCLIFlags(&c)...)
	flags = append(flags, natsCLIFlags(&c)...)
	flags = append(flags, rpcCLIFlags(&c, &headers)...)
	flags = append(flags, disconnectorCLIFlags(&c)...)
	flags = append(flags, logCLIFlags(&c)...)
	flags = append(flags, metricsCLIFlags(&c)...)
	flags = append(flags, wsCLIFlags(&c)...)
	flags = append(flags, pingCLIFlags(&c)...)
	flags = append(flags, jwtCLIFlags(&c)...)
	flags = append(flags, signedStreamsCLIFlags(&c)...)

	app := &cli.App{
		Name:            "anycable-go",
		Version:         version.Version(),
		Usage:           "AnyCable-Go, The WebSocket server for https://anycable.io",
		HideHelpCommand: true,
		Flags:           flags,
		Action: func(nc *cli.Context) error {
			helpOrVersionWereShown = false
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		return &config.Config{}, err, false
	}

	// helpOrVersionWereShown = false indicates that the default action has been run.
	// true means that help/version message was displayed.
	//
	// Unfortunately, cli module does not support another way of detecting if or which
	// command was run.
	if helpOrVersionWereShown {
		return &config.Config{}, nil, true
	}

	if path != "" {
		c.Path = strings.Split(path, " ")
	}

	c.Headers = strings.Split(strings.ToLower(headers), ",")

	if c.Debug {
		c.LogLevel = "debug"
		c.LogFormat = "text"
	}

	if c.Metrics.Port == 0 {
		c.Metrics.Port = c.Port
	}

	if c.Metrics.LogInterval > 0 {
		fmt.Println(`DEPRECATION WARNING: metrics_log_interval option is deprecated
and will be deleted in the next major release of anycable-go.
Use metrics_rotate_interval instead.`)

		if c.Metrics.RotateInterval == 0 {
			c.Metrics.RotateInterval = c.Metrics.LogInterval
		}
	}

	return &c, nil, false
}
