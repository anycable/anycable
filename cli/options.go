package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/config"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/version"
	"github.com/nats-io/nats.go"
	"github.com/urfave/cli/v2"
)

type cliOption func(*cli.App) error

type customOptionsFactory = func() ([]cli.Flag, error)

func WithCLIName(name string) cliOption {
	return func(app *cli.App) error {
		app.Name = name
		return nil
	}
}

func WithCLIVersion(str string) cliOption {
	return func(app *cli.App) error {
		app.Version = str
		return nil
	}
}

func WithCLIUsageHeader(desc string) cliOption {
	return func(app *cli.App) error {
		app.Usage = desc
		return nil
	}
}

func WithCLICustomOptions(factory customOptionsFactory) cliOption {
	return func(app *cli.App) error {
		custom, err := factory()
		if err != nil {
			return err
		}

		app.Flags = append(app.Flags, custom...)
		return nil
	}
}

// NewConfigFromCLI reads config from os.Args. It returns config, error (if any) and a bool value
// indicating that the usage message or version was shown, no further action required.
func NewConfigFromCLI(args []string, opts ...cliOption) (*config.Config, error, bool) {
	c := config.NewConfig()

	var path, headers, cookieFilter, mtags string
	var helpOrVersionWereShown = true
	var metricsFilter string
	var enatsRoutes, enatsGateways string
	var presets string

	// Print raw version without prefix
	cli.VersionPrinter = func(cCtx *cli.Context) {
		_, _ = fmt.Fprintf(cCtx.App.Writer, "%v\n", cCtx.App.Version)
	}

	flags := []cli.Flag{}
	flags = append(flags, serverCLIFlags(&c, &path)...)
	flags = append(flags, sslCLIFlags(&c)...)
	flags = append(flags, broadcastCLIFlags(&c)...)
	flags = append(flags, brokerCLIFlags(&c)...)
	flags = append(flags, redisCLIFlags(&c)...)
	flags = append(flags, httpBroadcastCLIFlags(&c)...)
	flags = append(flags, natsCLIFlags(&c)...)
	flags = append(flags, rpcCLIFlags(&c, &headers, &cookieFilter)...)
	flags = append(flags, disconnectorCLIFlags(&c)...)
	flags = append(flags, logCLIFlags(&c)...)
	flags = append(flags, metricsCLIFlags(&c, &metricsFilter, &mtags)...)
	flags = append(flags, wsCLIFlags(&c)...)
	flags = append(flags, pingCLIFlags(&c)...)
	flags = append(flags, jwtCLIFlags(&c)...)
	flags = append(flags, signedStreamsCLIFlags(&c)...)
	flags = append(flags, statsdCLIFlags(&c)...)
	flags = append(flags, embeddedNatsCLIFlags(&c, &enatsRoutes, &enatsGateways)...)
	flags = append(flags, miscCLIFlags(&c, &presets)...)

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

	for _, o := range opts {
		err := o(app)
		if err != nil {
			return &config.Config{}, err, false
		}
	}

	err := app.Run(args)
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
		c.Path = strings.Split(path, ",")
	}

	c.Headers = strings.Split(strings.ToLower(headers), ",")

	// Read session ID header if using a broker
	if c.BrokerAdapter != "" {
		c.Headers = append(c.Headers, broker.SESSION_ID_HEADER)
	}

	if len(cookieFilter) > 0 {
		c.Cookies = strings.Split(cookieFilter, ",")
	}

	if c.Debug {
		c.LogLevel = "debug"
		c.LogFormat = "text"
	}

	if c.Metrics.Port == 0 {
		c.Metrics.Port = c.Port
	}

	if mtags != "" {
		c.Metrics.Tags = parseTags(mtags)
	}

	if c.Metrics.LogInterval > 0 {
		fmt.Println(`DEPRECATION WARNING: metrics_log_interval option is deprecated
and will be deleted in the next major release of anycable-go.
Use metrics_rotate_interval instead.`)

		if c.Metrics.RotateInterval == 0 {
			c.Metrics.RotateInterval = c.Metrics.LogInterval
		}
	}

	if metricsFilter != "" {
		c.Metrics.LogFilter = strings.Split(metricsFilter, ",")
	}

	if enatsRoutes != "" {
		c.EmbeddedNats.Routes = strings.Split(enatsRoutes, ",")
	}

	if enatsGateways != "" {
		c.EmbeddedNats.Gateways = strings.Split(enatsGateways, ";")
	}

	if presets != "" {
		c.UserPresets = strings.Split(presets, ",")
	}

	// Automatically set the URL of the embedded NATS as the pub/sub server URL
	if c.EmbedNats && c.NATS.Servers == nats.DefaultURL {
		c.NATS.Servers = c.EmbeddedNats.ServiceAddr
	}

	if c.DisconnectorDisabled {
		fmt.Println(`DEPRECATION WARNING: disable_disconnect option is deprecated
and will be removed in the next major release of anycable-go.
Use disconnect_mode=never instead.`)

		c.App.DisconnectMode = node.DISCONNECT_MODE_NEVER
	}

	if c.DisconnectQueue.ShutdownTimeout > 0 {
		fmt.Println(`DEPRECATION WARNING: disconnect_timeout option is deprecated
and will be removed in the next major release of anycable-go.
Use shutdown_timeout instead.`)
	}

	return &c, nil, false
}

// Flags ordering issue: https://github.com/urfave/cli/pull/1430

const (
	serverCategoryDescription        = "ANYCABLE-GO SERVER:"
	sslCategoryDescription           = "SSL:"
	broadcastCategoryDescription     = "BROADCASTING:"
	redisCategoryDescription         = "REDIS:"
	httpBroadcastCategoryDescription = "HTTP BROADCAST:"
	natsCategoryDescription          = "NATS:"
	rpcCategoryDescription           = "RPC:"
	disconnectorCategoryDescription  = "DISCONNECTOR:"
	logCategoryDescription           = "LOG:"
	metricsCategoryDescription       = "METRICS:"
	wsCategoryDescription            = "WEBSOCKETS:"
	pingCategoryDescription          = "PING:"
	jwtCategoryDescription           = "JWT:"
	signedStreamsCategoryDescription = "SIGNED STREAMS:"
	statsdCategoryDescription        = "STATSD:"
	embeddedNatsCategoryDescription  = "EMBEDDED NATS:"
	miscCategoryDescription          = "MISC:"
	brokerCategoryDescription        = "BROKER:"

	envPrefix = "ANYCABLE_"
)

var (
	splitFlagName = regexp.MustCompile("[_-]")
)

// serverCLIFlags returns base server flags
func serverCLIFlags(c *config.Config, path *string) []cli.Flag {
	return withDefaults(serverCategoryDescription, []cli.Flag{
		&cli.StringFlag{
			Name:        "host",
			Value:       c.Host,
			Usage:       "Server host",
			Destination: &c.Host,
		},

		&cli.IntFlag{
			Name:        "port",
			Value:       c.Port,
			Usage:       "Server port",
			EnvVars:     []string{envPrefix + "PORT", "PORT"},
			Destination: &c.Port,
		},

		&cli.IntFlag{
			Name:        "max-conn",
			Usage:       "Limit simultaneous server connections (0 – without limit)",
			Destination: &c.MaxConn,
		},

		&cli.StringFlag{
			Name:        "path",
			Value:       strings.Join(c.Path, ","),
			Usage:       "WebSocket endpoint path (you can specify multiple paths using comma as separator)",
			Destination: path,
		},

		&cli.StringFlag{
			Name:        "health-path",
			Value:       c.HealthPath,
			Usage:       "HTTP health endpoint path",
			Destination: &c.HealthPath,
		},

		&cli.IntFlag{
			Name:        "shutdown_timeout",
			Usage:       "Graceful shutdown timeout (in seconds)",
			Value:       c.App.ShutdownTimeout,
			Destination: &c.App.ShutdownTimeout,
		},

		&cli.IntFlag{
			Name:        "shutdown_pool_size",
			Usage:       "The number of goroutines to use for disconnect calls on shutdown",
			Value:       c.App.ShutdownDisconnectPoolSize,
			Destination: &c.App.ShutdownDisconnectPoolSize,
			Hidden:      true,
		},
	})
}

// sslCLIFlags returns SSL flags
func sslCLIFlags(c *config.Config) []cli.Flag {
	return withDefaults(sslCategoryDescription, []cli.Flag{
		&cli.PathFlag{
			Name:        "ssl_cert",
			Usage:       "SSL certificate path",
			Destination: &c.SSL.CertPath,
		},

		&cli.PathFlag{
			Name:        "ssl_key",
			Usage:       "SSL private key path",
			Destination: &c.SSL.KeyPath,
		},
	})
}

// broadcastCLIFlags returns broadcast_adapter flag
func broadcastCLIFlags(c *config.Config) []cli.Flag {
	return withDefaults(broadcastCategoryDescription, []cli.Flag{
		&cli.StringFlag{
			Name:        "broadcast_adapter",
			Usage:       "Broadcasting adapter to use (http, redisx, redis or nats). You can specify multiple at once via a comma-separated list",
			Value:       c.BroadcastAdapter,
			Destination: &c.BroadcastAdapter,
		},
		&cli.StringFlag{
			Name:        "broker",
			Usage:       "Broker engine to use (memory)",
			Value:       c.BrokerAdapter,
			Destination: &c.BrokerAdapter,
		},
		&cli.StringFlag{
			Name:        "pubsub",
			Usage:       "Pub/Sub adapter to use (redis or nats)",
			Value:       c.PubSubAdapter,
			Destination: &c.PubSubAdapter,
		},
		&cli.IntFlag{
			Name:        "hub_gopool_size",
			Usage:       "The size of the goroutines pool to broadcast messages",
			Value:       c.App.HubGopoolSize,
			Destination: &c.App.HubGopoolSize,
		},
	})
}

// brokerCLIFlags returns broker related flags
func brokerCLIFlags(c *config.Config) []cli.Flag {
	return withDefaults(brokerCategoryDescription, []cli.Flag{
		&cli.IntFlag{
			Name:        "history_limit",
			Usage:       "Max number of messages to keep in the stream's history",
			Value:       c.Broker.HistoryLimit,
			Destination: &c.Broker.HistoryLimit,
		},
		&cli.Int64Flag{
			Name:        "history_ttl",
			Usage:       "TTL for messages in streams history (seconds)",
			Value:       c.Broker.HistoryTTL,
			Destination: &c.Broker.HistoryTTL,
		},
		&cli.Int64Flag{
			Name:        "sessions_ttl",
			Usage:       "TTL for expired/disconnected sessions (seconds)",
			Value:       c.Broker.SessionsTTL,
			Destination: &c.Broker.SessionsTTL,
		},
	})
}

func redisCLIFlags(c *config.Config) []cli.Flag {
	return withDefaults(redisCategoryDescription, []cli.Flag{
		&cli.StringFlag{
			Name:        "redis_url",
			Usage:       "Redis url",
			Value:       c.Redis.URL,
			EnvVars:     []string{envPrefix + "REDIS_URL", "REDIS_URL"},
			Destination: &c.Redis.URL,
		},

		&cli.StringFlag{
			Name:        "redis_channel",
			Usage:       "Redis channel for broadcasts",
			Value:       c.Redis.Channel,
			Destination: &c.Redis.Channel,
		},

		&cli.StringFlag{
			Name:        "redis_sentinels",
			Usage:       "Comma separated list of sentinel hosts, format: 'hostname:port,..'",
			Destination: &c.Redis.Sentinels,
		},

		&cli.IntFlag{
			Name:        "redis_sentinel_discovery_interval",
			Usage:       "Interval to rediscover sentinels in seconds",
			Value:       c.Redis.SentinelDiscoveryInterval,
			Destination: &c.Redis.SentinelDiscoveryInterval,
		},

		&cli.IntFlag{
			Name:        "redis_keepalive_interval",
			Usage:       "Interval to periodically ping Redis to make sure it's alive",
			Value:       c.Redis.KeepalivePingInterval,
			Destination: &c.Redis.KeepalivePingInterval,
		},

		&cli.BoolFlag{
			Name:        "redis_tls_verify",
			Usage:       "Verify Redis server TLS certificate (only if URL protocol is rediss://)",
			Value:       c.Redis.TLSVerify,
			Destination: &c.Redis.TLSVerify,
		},
	})
}

// httpBroadcastCLIFlags returns HTTP CLI flags
func httpBroadcastCLIFlags(c *config.Config) []cli.Flag {
	return withDefaults(httpBroadcastCategoryDescription, []cli.Flag{
		&cli.IntFlag{
			Name:        "http_broadcast_port",
			Usage:       "HTTP pub/sub server port",
			Value:       c.HTTPBroadcast.Port,
			Destination: &c.HTTPBroadcast.Port,
		},

		&cli.StringFlag{
			Name:        "http_broadcast_path",
			Usage:       "HTTP pub/sub endpoint path",
			Value:       c.HTTPBroadcast.Path,
			Destination: &c.HTTPBroadcast.Path,
		},

		&cli.StringFlag{
			Name:        "http_broadcast_secret",
			Usage:       "HTTP pub/sub authorization secret",
			Destination: &c.HTTPBroadcast.Secret,
		},
	})
}

// natsCLIFlags returns NATS cli flags
func natsCLIFlags(c *config.Config) []cli.Flag {
	return withDefaults(natsCategoryDescription, []cli.Flag{
		&cli.StringFlag{
			Name:        "nats_servers",
			Usage:       "Comma separated list of NATS cluster servers",
			Value:       c.NATS.Servers,
			Destination: &c.NATS.Servers,
		},

		&cli.StringFlag{
			Name:        "nats_channel",
			Usage:       "NATS channel for broadcasts",
			Value:       c.NATS.Channel,
			Destination: &c.NATS.Channel,
		},

		&cli.BoolFlag{
			Name:        "nats_dont_randomize_servers",
			Usage:       "Pass this option to disable NATS servers randomization during (re-)connect",
			Destination: &c.NATS.DontRandomizeServers,
		},
	})
}

// embeddedNatsCLIFlags returns NATS cli flags
func embeddedNatsCLIFlags(c *config.Config, routes *string, gateways *string) []cli.Flag {
	return withDefaults(embeddedNatsCategoryDescription, []cli.Flag{
		&cli.BoolFlag{
			Name:        "embed_nats",
			Usage:       "Enable embedded NATS server and use it for pub/sub",
			Value:       c.EmbedNats,
			Destination: &c.EmbedNats,
		},

		&cli.StringFlag{
			Name:        "enats_addr",
			Usage:       "NATS server bind address",
			Value:       c.EmbeddedNats.ServiceAddr,
			Destination: &c.EmbeddedNats.ServiceAddr,
		},

		&cli.StringFlag{
			Name:        "enats_cluster",
			Usage:       "NATS cluster service bind address",
			Value:       c.EmbeddedNats.ClusterAddr,
			Destination: &c.EmbeddedNats.ClusterAddr,
		},

		&cli.StringFlag{
			Name:        "enats_cluster_name",
			Usage:       "NATS cluster name",
			Value:       c.EmbeddedNats.ClusterName,
			Destination: &c.EmbeddedNats.ClusterName,
		},

		&cli.StringFlag{
			Name:        "enats_cluster_routes",
			Usage:       "Comma-separated list of known cluster addresses",
			Destination: routes,
		},

		&cli.StringFlag{
			Name:        "enats_gateway",
			Usage:       "NATS gateway bind address",
			Value:       c.EmbeddedNats.GatewayAddr,
			Destination: &c.EmbeddedNats.GatewayAddr,
		},

		&cli.StringFlag{
			Name:        "enats_gateways",
			Usage:       "Semicolon-separated list of known gateway configurations: name_a:gateway_1,gateway_2;name_b:gateway_4",
			Destination: gateways,
		},

		&cli.StringFlag{
			Name:        "enats_gateway_advertise",
			Usage:       "NATS gateway advertise address",
			Value:       c.EmbeddedNats.GatewayAdvertise,
			Destination: &c.EmbeddedNats.GatewayAdvertise,
		},

		&cli.BoolFlag{
			Name:        "enats_debug",
			Usage:       "Enable NATS server logs",
			Destination: &c.EmbeddedNats.Debug,
		},

		&cli.BoolFlag{
			Name:        "enats_trace",
			Usage:       "Enable NATS server protocol trace logs",
			Destination: &c.EmbeddedNats.Trace,
		},
	})
}

// rpcCLIFlags returns CLI flags for RPC
func rpcCLIFlags(c *config.Config, headers, cookieFilter *string) []cli.Flag {
	return withDefaults(rpcCategoryDescription, []cli.Flag{
		&cli.StringFlag{
			Name:        "rpc_host",
			Usage:       "RPC service address (full URL in case of HTTP RPC)",
			Value:       c.RPC.Host,
			Destination: &c.RPC.Host,
		},

		&cli.IntFlag{
			Name:        "rpc_concurrency",
			Usage:       "Max number of concurrent RPC request; should be slightly less than the RPC server concurrency",
			Value:       c.RPC.Concurrency,
			Destination: &c.RPC.Concurrency,
		},

		&cli.BoolFlag{
			Name:        "rpc_enable_tls",
			Usage:       "Enable client-side TLS with the RPC server",
			Destination: &c.RPC.EnableTLS,
		},

		&cli.BoolFlag{
			Name:        "rpc_tls_verify",
			Usage:       "Whether to verify the RPC server certificate",
			Destination: &c.RPC.TLSVerify,
			Value:       true,
		},

		&cli.StringFlag{
			Name:        "rpc_tls_root_ca",
			Usage:       "CA root certificate file path or contents in PEM format (if not set, system CAs will be used)",
			Destination: &c.RPC.TLSRootCA,
		},

		&cli.IntFlag{
			Name:        "rpc_max_call_recv_size",
			Usage:       "Override default MaxCallRecvMsgSize for RPC client (bytes)",
			Value:       c.RPC.MaxRecvSize,
			Destination: &c.RPC.MaxRecvSize,
		},

		&cli.IntFlag{
			Name:        "rpc_max_call_send_size",
			Usage:       "Override default MaxCallSendMsgSize for RPC client (bytes)",
			Value:       c.RPC.MaxSendSize,
			Destination: &c.RPC.MaxSendSize,
		},

		&cli.StringFlag{
			Name:        "headers",
			Usage:       "List of headers to proxy to RPC",
			Value:       strings.Join(c.Headers, ","),
			Destination: headers,
		},

		&cli.StringFlag{
			Name:        "proxy-cookies",
			Usage:       "Cookie keys to send to RPC, default is all",
			Destination: cookieFilter,
		},

		&cli.StringFlag{
			Name:        "rpc_impl",
			Usage:       "RPC implementation (grpc, http)",
			Value:       c.RPC.Implementation,
			Destination: &c.RPC.Implementation,
		},

		&cli.StringFlag{
			Name:        "http_rpc_secret",
			Usage:       "Authentication secret for RPC over HTTP",
			Value:       c.RPC.Secret,
			Destination: &c.RPC.Secret,
		},

		&cli.IntFlag{
			Name:        "http_rpc_timeout",
			Usage:       "HTTP RPC timeout (in ms)",
			Value:       c.RPC.RequestTimeout,
			Destination: &c.RPC.RequestTimeout,
		},
	})
}

// rpcCLIFlags returns CLI flags for disconnect options
func disconnectorCLIFlags(c *config.Config) []cli.Flag {
	return withDefaults(disconnectorCategoryDescription, []cli.Flag{
		&cli.StringFlag{
			Name:        "disconnect_mode",
			Usage:       "Define when to call Disconnect callback (always, never, auto)",
			Destination: &c.App.DisconnectMode,
			Value:       c.App.DisconnectMode,
		},

		&cli.IntFlag{
			Name:        "disconnect_rate",
			Usage:       "Max number of Disconnect calls per second",
			Value:       c.DisconnectQueue.Rate,
			Destination: &c.DisconnectQueue.Rate,
		},

		&cli.IntFlag{
			Name:        "disconnect_timeout",
			Usage:       "[DEPRECATED] Graceful shutdown timeout (in seconds)",
			Value:       c.DisconnectQueue.ShutdownTimeout,
			Destination: &c.DisconnectQueue.ShutdownTimeout,
			Hidden:      true,
		},

		&cli.BoolFlag{
			Name:        "disable_disconnect",
			Usage:       "Disable calling Disconnect callback",
			Destination: &c.DisconnectorDisabled,
			Hidden:      true,
		},
	})
}

// rpcCLIFlags returns CLI flags for logging
func logCLIFlags(c *config.Config) []cli.Flag {
	return withDefaults(logCategoryDescription, []cli.Flag{
		&cli.StringFlag{
			Name:        "log_level",
			Usage:       "Set logging level (debug/info/warn/error/fatal)",
			Value:       c.LogLevel,
			Destination: &c.LogLevel,
		},

		&cli.StringFlag{
			Name:        "log_format",
			Usage:       "Set logging format (text/json)",
			Value:       c.LogFormat,
			Destination: &c.LogFormat,
		},

		&cli.BoolFlag{
			Name:        "debug",
			Usage:       "Enable debug mode (more verbose logging)",
			Destination: &c.Debug,
		},
	})
}

// metricsCLIFlags returns CLI flags for metrics
func metricsCLIFlags(c *config.Config, filter *string, mtags *string) []cli.Flag {
	return withDefaults(metricsCategoryDescription, []cli.Flag{
		// Metrics
		&cli.BoolFlag{
			Name:        "metrics_log",
			Usage:       "Enable metrics logging (with info level)",
			Destination: &c.Metrics.Log,
		},

		&cli.IntFlag{
			Name:        "metrics_rotate_interval",
			Usage:       "Specify how often flush metrics to writers (logs, statsd) (in seconds)",
			Value:       c.Metrics.RotateInterval,
			Destination: &c.Metrics.RotateInterval,
		},

		&cli.IntFlag{
			Name:        "metrics_log_interval",
			Usage:       "DEPRECATED. Specify how often flush metrics logs (in seconds)",
			Value:       c.Metrics.LogInterval,
			Destination: &c.Metrics.LogInterval,
		},

		&cli.StringFlag{
			Name:        "metrics_log_filter",
			Usage:       "Specify list of metrics to print to log (to reduce the output)",
			Destination: filter,
		},

		&cli.StringFlag{
			Name:        "metrics_log_formatter",
			Usage:       "Specify the path to custom Ruby formatter script (only supported on MacOS and Linux)",
			Destination: &c.Metrics.LogFormatter,
		},

		&cli.StringFlag{
			Name:        "metrics_http",
			Usage:       "Enable HTTP metrics endpoint at the specified path",
			Destination: &c.Metrics.HTTP,
		},

		&cli.StringFlag{
			Name:        "metrics_host",
			Usage:       "Server host for metrics endpoint",
			Destination: &c.Metrics.Host,
		},

		&cli.IntFlag{
			Name:        "metrics_port",
			Usage:       "Server port for metrics endpoint, the same as for main server by default",
			Destination: &c.Metrics.Port,
		},

		&cli.StringFlag{
			Name:        "metrics_tags",
			Usage:       "Comma-separated list of default (global) tags to add to every metric",
			Destination: mtags,
		},

		&cli.IntFlag{
			Name:        "stats_refresh_interval",
			Usage:       "How often to refresh the server stats (in seconds)",
			Value:       c.App.StatsRefreshInterval,
			Destination: &c.App.StatsRefreshInterval,
		},
	})
}

// wsCLIFlags returns CLI flags for WebSocket
func wsCLIFlags(c *config.Config) []cli.Flag {
	return withDefaults(wsCategoryDescription, []cli.Flag{
		&cli.IntFlag{
			Name:        "read_buffer_size",
			Usage:       "WebSocket connection read buffer size",
			Value:       c.WS.ReadBufferSize,
			Destination: &c.WS.ReadBufferSize,
		},

		&cli.IntFlag{
			Name:        "write_buffer_size",
			Usage:       "WebSocket connection write buffer size",
			Value:       c.WS.WriteBufferSize,
			Destination: &c.WS.WriteBufferSize,
		},

		&cli.Int64Flag{
			Name:        "max_message_size",
			Usage:       "Maximum size of a message in bytes",
			Value:       c.WS.MaxMessageSize,
			Destination: &c.WS.MaxMessageSize,
		},

		&cli.BoolFlag{
			Name:        "enable_ws_compression",
			Usage:       "Enable experimental WebSocket per message compression",
			Destination: &c.WS.EnableCompression,
		},

		&cli.StringFlag{
			Name:        "allowed_origins",
			Usage:       `Accept requests only from specified origins, e.g., "www.example.com,*example.io". No check is performed if empty`,
			Destination: &c.WS.AllowedOrigins,
		},
	})
}

// pingCLIFlags returns CLI flag for ping settings
func pingCLIFlags(c *config.Config) []cli.Flag {
	return withDefaults(pingCategoryDescription, []cli.Flag{
		&cli.IntFlag{
			Name:        "ping_interval",
			Usage:       "Action Cable ping interval (in seconds)",
			Value:       c.App.PingInterval,
			Destination: &c.App.PingInterval,
		},

		&cli.StringFlag{
			Name:        "ping_timestamp_precision",
			Usage:       "Precision for timestamps in ping messages (s, ms, ns)",
			Value:       c.App.PingTimestampPrecision,
			Destination: &c.App.PingTimestampPrecision,
		},
	})
}

// jwtCLIFlags returns CLI flags for JWT
func jwtCLIFlags(c *config.Config) []cli.Flag {
	return withDefaults(jwtCategoryDescription, []cli.Flag{
		&cli.StringFlag{
			Name:        "jwt_id_key",
			Usage:       "The encryption key used to verify JWT tokens",
			Destination: &c.JWT.Secret,
		},

		&cli.StringFlag{
			Name:        "jwt_id_param",
			Usage:       "The name of a query string param or an HTTP header carrying a token",
			Value:       c.JWT.Param,
			Destination: &c.JWT.Param,
		},

		&cli.BoolFlag{
			Name:        "jwt_id_enforce",
			Usage:       "Whether to enforce token presence for all connections",
			Destination: &c.JWT.Force,
		},
	})
}

// signedStreamsCLIFlags returns misc CLI flags
func signedStreamsCLIFlags(c *config.Config) []cli.Flag {
	return withDefaults(signedStreamsCategoryDescription, []cli.Flag{
		&cli.StringFlag{
			Name:        "turbo_rails_key",
			Usage:       "Enable Turbo Streams fastlane with the specified signing key",
			Destination: &c.Rails.TurboRailsKey,
		},

		&cli.BoolFlag{
			Name:        "turbo_rails_cleartext",
			Usage:       "Enable Turbo Streams fastlane without stream names signing",
			Destination: &c.Rails.TurboRailsClearText,
		},

		&cli.StringFlag{
			Name:        "cable_ready_key",
			Usage:       "Enable CableReady fastlane with the specified signing key",
			Destination: &c.Rails.CableReadyKey,
		},

		&cli.BoolFlag{
			Name:        "cable_ready_cleartext",
			Usage:       "Enable Cable Ready fastlane without stream names signing",
			Destination: &c.Rails.CableReadyClearText,
		},
	})
}

// StatsD related flags
func statsdCLIFlags(c *config.Config) []cli.Flag {
	return withDefaults(statsdCategoryDescription, []cli.Flag{
		&cli.StringFlag{
			Name:        "statsd_host",
			Usage:       "Server host for metrics sent to statsd server in the format <host>:<port>",
			Destination: &c.Metrics.Statsd.Host,
		},
		&cli.StringFlag{
			Name:        "statsd_prefix",
			Usage:       "Statsd metrics prefix",
			Value:       c.Metrics.Statsd.Prefix,
			Destination: &c.Metrics.Statsd.Prefix,
		},
		&cli.IntFlag{
			Name:        "statsd_max_packet_size",
			Usage:       "Statsd client maximum UDP packet size",
			Value:       c.Metrics.Statsd.MaxPacketSize,
			Destination: &c.Metrics.Statsd.MaxPacketSize,
		},
		&cli.StringFlag{
			Name:        "statsd_tags_format",
			Usage:       `One of "datadog", "influxdb", or "graphite"`,
			Value:       c.Metrics.Statsd.TagFormat,
			Destination: &c.Metrics.Statsd.TagFormat,
		},
	})
}

// miscCLIFlags returns uncategorized flags
func miscCLIFlags(c *config.Config, presets *string) []cli.Flag {
	return withDefaults(miscCategoryDescription, []cli.Flag{
		&cli.StringFlag{
			Name:        "presets",
			Usage:       "Configuration presets, comma-separated (none, fly, heroku, broker). Inferred automatically",
			Destination: presets,
		},
	})
}

// withDefaults sets category and env var name a flags passed as the arument
func withDefaults(category string, flags []cli.Flag) []cli.Flag {
	for _, f := range flags {
		switch v := f.(type) {
		case *cli.IntFlag:
			v.Category = category
			if len(v.EnvVars) == 0 {
				v.EnvVars = []string{nameToEnvVarName(v.Name)}
			}
		case *cli.Int64Flag:
			v.Category = category
			if len(v.EnvVars) == 0 {
				v.EnvVars = []string{nameToEnvVarName(v.Name)}
			}
		case *cli.Float64Flag:
			v.Category = category
			if len(v.EnvVars) == 0 {
				v.EnvVars = []string{nameToEnvVarName(v.Name)}
			}
		case *cli.DurationFlag:
			v.Category = category
			if len(v.EnvVars) == 0 {
				v.EnvVars = []string{nameToEnvVarName(v.Name)}
			}
		case *cli.BoolFlag:
			v.Category = category
			if len(v.EnvVars) == 0 {
				v.EnvVars = []string{nameToEnvVarName(v.Name)}
			}
		case *cli.StringFlag:
			v.Category = category
			if len(v.EnvVars) == 0 {
				v.EnvVars = []string{nameToEnvVarName(v.Name)}
			}
		case *cli.PathFlag:
			v.Category = category
			if len(v.EnvVars) == 0 {
				v.EnvVars = []string{nameToEnvVarName(v.Name)}
			}
		case *cli.TimestampFlag:
			v.Category = category
			if len(v.EnvVars) == 0 {
				v.EnvVars = []string{nameToEnvVarName(v.Name)}
			}
		}
	}
	return flags
}

// nameToEnvVarName converts flag name to env variable
func nameToEnvVarName(name string) string {
	split := splitFlagName.Split(name, -1)
	set := []string{}

	for i := range split {
		set = append(set, strings.ToUpper(split[i]))
	}

	return envPrefix + strings.Join(set, "_")
}

func parseTags(str string) map[string]string {
	tags := strings.Split(str, ",")

	res := make(map[string]string, len(tags))

	for _, v := range tags {
		parts := strings.Split(v, ":")
		res[parts[0]] = parts[1]
	}

	return res
}
