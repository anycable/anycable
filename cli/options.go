package cli

import (
	"fmt"
	"regexp"
	"strings"

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
	var turboRailsKey, cableReadyKey string
	var turboRailsClearText, cableReadyClearText bool
	var jwtIdKey, jwtIdParam string
	var jwtIdEnforce bool
	var noRPC bool
	var isPublic bool

	// Print raw version without prefix
	cli.VersionPrinter = func(cCtx *cli.Context) {
		_, _ = fmt.Fprintf(cCtx.App.Writer, "%v\n", cCtx.App.Version)
	}

	flags := []cli.Flag{}
	flags = append(flags, serverCLIFlags(&c, &path, &isPublic)...)
	flags = append(flags, sslCLIFlags(&c)...)
	flags = append(flags, broadcastCLIFlags(&c)...)
	flags = append(flags, brokerCLIFlags(&c)...)
	flags = append(flags, redisCLIFlags(&c)...)
	flags = append(flags, httpBroadcastCLIFlags(&c)...)
	flags = append(flags, natsCLIFlags(&c)...)
	flags = append(flags, rpcCLIFlags(&c, &headers, &cookieFilter, &noRPC)...)
	flags = append(flags, disconnectorCLIFlags(&c)...)
	flags = append(flags, logCLIFlags(&c)...)
	flags = append(flags, metricsCLIFlags(&c, &metricsFilter, &mtags)...)
	flags = append(flags, wsCLIFlags(&c)...)
	flags = append(flags, pingCLIFlags(&c)...)
	flags = append(flags, jwtCLIFlags(&c, &jwtIdKey, &jwtIdParam, &jwtIdEnforce)...)
	flags = append(flags, signedStreamsCLIFlags(&c, &turboRailsKey, &cableReadyKey, &turboRailsClearText, &cableReadyClearText)...)
	flags = append(flags, statsdCLIFlags(&c)...)
	flags = append(flags, graphqlCLIFlags(&c)...)
	flags = append(flags, embeddedNatsCLIFlags(&c, &enatsRoutes, &enatsGateways)...)
	flags = append(flags, sseCLIFlags(&c)...)
	flags = append(flags, ocppCLIFlags(&c)...)
	flags = append(flags, miscCLIFlags(&c, &presets)...)
	flags = append(flags, longPollingCLIFlags(&c)...)

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

	if len(cookieFilter) > 0 {
		c.Cookies = strings.Split(cookieFilter, ",")
	}

	if c.Debug {
		c.LogLevel = "debug"
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

	c.SSE.AllowedOrigins = c.WS.AllowedOrigins
	c.HTTPBroadcast.CORSHosts = c.WS.AllowedOrigins

	if turboRailsKey != "" {
		fmt.Println(`DEPRECATION WARNING: turbo_rails_key option is deprecated
and will be removed in the next major release of anycable-go.
Use turbo_streams_secret instead.`)

		c.Streams.TurboSecret = turboRailsKey

		c.Streams.Turbo = true
	}

	if turboRailsClearText {
		fmt.Println(`DEPRECATION WARNING: turbo_rails_cleartext option is deprecated
and will be removed in the next major release of anycable-go.
It has no effect anymore, use public streams instead.`)
	}

	if cableReadyKey != "" {
		fmt.Println(`DEPRECATION WARNING: cable_ready_key option is deprecated
and will be removed in the next major release of anycable-go.
Use cable_ready_secret instead.`)

		c.Streams.CableReadySecret = cableReadyKey

		c.Streams.CableReady = true
	}

	if cableReadyClearText {
		fmt.Println(`DEPRECATION WARNING: cable_ready_cleartext option is deprecated
and will be removed in the next major release of anycable-go.
It has no effect anymore, use public streams instead.`)
	}

	if jwtIdKey != "" {
		fmt.Println(`DEPRECATION WARNING: jwt_id_key option is deprecated
and will be removed in the next major release of anycable-go.
Use jwt_secret instead.`)

		if c.JWT.Secret == "" {
			c.JWT.Secret = jwtIdKey
		}
	}

	if jwtIdParam != "" {
		fmt.Println(`DEPRECATION WARNING: jwt_id_param option is deprecated
and will be removed in the next major release of anycable-go.
Use jwt_param instead.`)

		if c.JWT.Param == "" {
			c.JWT.Param = jwtIdParam
		}
	}

	if jwtIdEnforce {
		fmt.Println(`DEPRECATION WARNING: jwt_id_enforce option is deprecated
and will be removed in the next major release of anycable-go.
Use enfore_jwt instead.`)

		c.JWT.Force = true
	}

	// Configure RPC
	if noRPC {
		c.RPC.Implementation = "none"
	}

	// Legacy HTTP authentication stuff
	if c.HTTPBroadcast.Secret != "" {
		fmt.Println(`DEPRECATION WARNING: http_broadcast_secret option is deprecated
and will be removed in the next major release of anycable-go.
Use broadcast_key instead.`)
	}

	if c.HTTPBroadcast.Secret == "" {
		c.HTTPBroadcast.Secret = c.BroadcastKey
	}

	// Fallback secrets
	if c.Secret != "" {
		if c.Streams.Secret == "" {
			c.Streams.Secret = c.Secret
		}

		if c.JWT.Secret == "" {
			c.JWT.Secret = c.Secret
		}

		if c.HTTPBroadcast.Secret == "" {
			c.HTTPBroadcast.SecretBase = c.Secret
		}

		if c.RPC.Secret == "" {
			c.RPC.SecretBase = c.Secret
		}
	}

	// Nullify none secrets
	if c.Streams.Secret == "none" {
		c.Streams.Secret = ""
	}

	if c.JWT.Secret == "none" {
		c.JWT.Secret = ""
	}

	if c.RPC.Secret == "none" {
		c.RPC.Secret = ""
	}

	if c.HTTPBroadcast.Secret == "none" {
		c.HTTPBroadcast.Secret = ""
	}

	// Configure default HTTP port
	if c.HTTPBroadcast.Port == 0 {
		if c.HTTPBroadcast.IsSecured() {
			c.HTTPBroadcast.Port = c.Port
		} else {
			c.HTTPBroadcast.Port = 8090
		}
	}

	// Configure public mode and other insecure features
	if isPublic {
		c.SkipAuth = true
		c.Streams.Public = true
		// Ensure broadcasting is also public
		c.HTTPBroadcast.Secret = ""
		c.HTTPBroadcast.SecretBase = ""
	}

	if (c.LegacyGraphQL.Path) != "" {
		fmt.Println(`DEPRECATION WARNING: apollo_path option is deprecated
and will be deleted in the next major release of anycable-go.
Use graphql_path instead.`)

		c.GraphQL.Path = c.LegacyGraphQL.Path
	}

	if (c.LegacyGraphQL.Channel) != "" {
		fmt.Println(`DEPRECATION WARNING: apollo_channel option is deprecated
and will be deleted in the next major release of anycable-go.
Use graphql_channel instead.`)

		c.GraphQL.Channel = c.LegacyGraphQL.Channel
	}

	if (c.LegacyGraphQL.Action) != "" {
		fmt.Println(`DEPRECATION WARNING: apollo_action option is deprecated
and will be deleted in the next major release of anycable-go.
Use graphql_action instead.`)

		c.GraphQL.Action = c.LegacyGraphQL.Action
	}

	if c.JWT.Enabled() {
		c.GraphQL.JWTParam = c.JWT.Param
	}

	// Inherit allowed origins from WS config
	c.LongPolling.AllowedOrigins = c.WS.AllowedOrigins

	return &c, nil, false
}

// NewConfig returns a new AnyCable configuration combining default values and values from the environment.
func NewConfig() *config.Config {
	c, err, _ := NewConfigFromCLI([]string{})

	if err != nil {
		panic(err)
	}

	return c
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
	sseCategoryDescription           = "SERVER-SENT EVENTS:"
	graphqlCategoryDescription       = "GRAPHQL:"
	ocppCategoryDescription          = "OCCP:"
	longPollingCategoryDescription   = "LONG POLLING:"

	envPrefix = "ANYCABLE_"
)

var (
	splitFlagName = regexp.MustCompile("[_-]")
)

// serverCLIFlags returns base server flags
func serverCLIFlags(c *config.Config, path *string, isPublic *bool) []cli.Flag {
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

		&cli.StringFlag{
			Name:        "secret",
			Usage:       "A common secret key used by all features by default",
			Value:       c.Secret,
			Destination: &c.Secret,
		},

		&cli.StringFlag{
			Name:        "broadcast_key",
			Usage:       "An authentication key for broadcast requests",
			Value:       c.BroadcastKey,
			Destination: &c.BroadcastKey,
		},

		&cli.BoolFlag{
			Name:        "public",
			Usage:       "[DANGER ZONE] Run server in the public mode allowing all connections and stream subscriptions",
			Destination: isPublic,
		},

		&cli.BoolFlag{
			Name:        "noauth",
			Usage:       "[DANGER ZONE] Disable client authentication over RPC",
			Value:       c.SkipAuth,
			Destination: &c.SkipAuth,
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

		&cli.BoolFlag{
			Name:        "shutdown_slowdrain",
			Usage:       "Enable slow draining of connections during shutdown",
			Value:       c.App.SlowDrainShutdown,
			Destination: &c.App.SlowDrainShutdown,
		},

		&cli.IntFlag{
			Name:        "shutdown_slowdrain_max_interval",
			Usage:       "The max interval between slow drain disconnect calls (milliseconds)",
			Value:       c.App.SlowDrainMaxInterval,
			Destination: &c.App.SlowDrainMaxInterval,
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
			Usage:       "Broker engine to use (memory or redis)",
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
			Hidden:      true,
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
			Hidden:      true,
		},

		&cli.IntFlag{
			Name:        "redis_keepalive_interval",
			Usage:       "Interval to periodically ping Redis to make sure it's alive",
			Value:       c.Redis.KeepalivePingInterval,
			Destination: &c.Redis.KeepalivePingInterval,
			Hidden:      true,
		},

		&cli.BoolFlag{
			Name:        "redis_tls_verify",
			Usage:       "Verify Redis server TLS certificate (only if URL protocol is rediss://)",
			Value:       c.Redis.TLSVerify,
			Destination: &c.Redis.TLSVerify,
			Hidden:      true,
		},

		&cli.BoolFlag{
			Name:        "redis_disable_cache",
			Usage:       "Disable client-side caching",
			Value:       c.Redis.DisableCache,
			Destination: &c.Redis.DisableCache,
			Hidden:      true,
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
			Usage:       "[Deprecated] HTTP pub/sub authorization secret",
			Destination: &c.HTTPBroadcast.Secret,
			Hidden:      true,
		},

		&cli.BoolFlag{
			Name:        "http_broadcast_cors",
			Destination: &c.HTTPBroadcast.AddCORSHeaders,
			Hidden:      true,
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
			Hidden:      true,
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
			Hidden:      true,
		},

		&cli.StringFlag{
			Name:        "enats_cluster",
			Usage:       "NATS cluster service bind address",
			Value:       c.EmbeddedNats.ClusterAddr,
			Destination: &c.EmbeddedNats.ClusterAddr,
			Hidden:      true,
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
			Hidden:      true,
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

		&cli.StringFlag{
			Name:        "enats_store_dir",
			Usage:       "Embedded NATS store directory (for JetStream)",
			Value:       c.EmbeddedNats.StoreDir,
			Destination: &c.EmbeddedNats.StoreDir,
			Hidden:      true,
		},

		&cli.StringFlag{
			Name:        "enats_server_name",
			Usage:       "Embedded NATS unique server name (required for JetStream), auto-generated by default",
			Value:       c.EmbeddedNats.Name,
			Destination: &c.EmbeddedNats.Name,
		},

		&cli.BoolFlag{
			Name:        "enats_debug",
			Usage:       "Enable NATS server logs",
			Destination: &c.EmbeddedNats.Debug,
			Hidden:      true,
		},

		&cli.BoolFlag{
			Name:        "enats_trace",
			Usage:       "Enable NATS server protocol trace logs",
			Destination: &c.EmbeddedNats.Trace,
			Hidden:      true,
		},
	})
}

// rpcCLIFlags returns CLI flags for RPC
func rpcCLIFlags(c *config.Config, headers, cookieFilter *string, isNone *bool) []cli.Flag {
	return withDefaults(rpcCategoryDescription, []cli.Flag{
		&cli.StringFlag{
			Name:        "rpc_host",
			Usage:       "RPC service address (full URL in case of HTTP RPC)",
			Value:       c.RPC.Host,
			Destination: &c.RPC.Host,
		},

		&cli.BoolFlag{
			Name:        "norpc",
			Usage:       "Disable RPC component and run server in the standalone mode",
			Destination: isNone,
		},

		&cli.IntFlag{
			Name:        "rpc_concurrency",
			Usage:       "Max number of concurrent RPC requests (set to 0 to enable adaptive concurrency)",
			Value:       c.RPC.Concurrency,
			Destination: &c.RPC.Concurrency,
		},

		&cli.IntFlag{
			Name:        "rpc_concurrency_initial",
			Usage:       "Initial concurrency (when adaptive concurrency is enabled)",
			Value:       c.RPC.Smart.InitialConcurrency,
			Destination: &c.RPC.Smart.InitialConcurrency,
		},

		&cli.IntFlag{
			Name:        "rpc_concurrency_max",
			Usage:       "Max concurrency (when adaptive concurrency is enabled)",
			Value:       c.RPC.Smart.MaxConcurrency,
			Destination: &c.RPC.Smart.MaxConcurrency,
		},

		&cli.IntFlag{
			Name:        "rpc_concurrency_min",
			Usage:       "Min concurrency (when adaptive concurrency is enabled)",
			Value:       c.RPC.Smart.MinConcurrency,
			Destination: &c.RPC.Smart.MinConcurrency,
		},

		&cli.BoolFlag{
			Name:        "rpc_enable_tls",
			Usage:       "Enable client-side TLS with the RPC server",
			Destination: &c.RPC.EnableTLS,
			Hidden:      true,
		},

		&cli.BoolFlag{
			Name:        "rpc_tls_verify",
			Usage:       "Whether to verify the RPC server certificate",
			Destination: &c.RPC.TLSVerify,
			Value:       true,
			Hidden:      true,
		},

		&cli.StringFlag{
			Name:        "rpc_tls_root_ca",
			Usage:       "CA root certificate file path or contents in PEM format (if not set, system CAs will be used)",
			Destination: &c.RPC.TLSRootCA,
			Hidden:      true,
		},

		&cli.IntFlag{
			Name:        "rpc_max_call_recv_size",
			Usage:       "Override default MaxCallRecvMsgSize for RPC client (bytes)",
			Value:       c.RPC.MaxRecvSize,
			Destination: &c.RPC.MaxRecvSize,
			Hidden:      true,
		},

		&cli.IntFlag{
			Name:        "rpc_max_call_send_size",
			Usage:       "Override default MaxCallSendMsgSize for RPC client (bytes)",
			Value:       c.RPC.MaxSendSize,
			Destination: &c.RPC.MaxSendSize,
			Hidden:      true,
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
			Hidden:      true,
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
			Hidden:      true,
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
			Hidden:      true,
		},

		&cli.IntFlag{
			Name:        "disconnect_backlog_size",
			Usage:       "The size of the channel's buffer for disconnect requests",
			Value:       c.DisconnectQueue.Backlog,
			Destination: &c.DisconnectQueue.Backlog,
			Hidden:      true,
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
			Usage:       "Set logging level (debug/info/warn/error)",
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
			Hidden:      true,
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
			Hidden:      true,
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
			Hidden:      true,
		},

		&cli.IntFlag{
			Name:        "write_buffer_size",
			Usage:       "WebSocket connection write buffer size",
			Value:       c.WS.WriteBufferSize,
			Destination: &c.WS.WriteBufferSize,
			Hidden:      true,
		},

		&cli.Int64Flag{
			Name:        "max_message_size",
			Usage:       "Maximum size of a message in bytes",
			Value:       c.WS.MaxMessageSize,
			Destination: &c.WS.MaxMessageSize,
			Hidden:      true,
		},

		&cli.BoolFlag{
			Name:        "enable_ws_compression",
			Usage:       "Enable experimental WebSocket per message compression",
			Destination: &c.WS.EnableCompression,
			Hidden:      true,
		},

		&cli.StringFlag{
			Name:        "allowed_origins",
			Usage:       `Accept requests only from specified origins, e.g., "www.example.com,*example.io". No check is performed if empty`,
			Destination: &c.WS.AllowedOrigins,
		},

		&cli.IntFlag{
			Name:        "read_gopool_size",
			Usage:       "The size of the goroutine pool to read client messages",
			Value:       c.App.ReadGopoolSize,
			Destination: &c.App.ReadGopoolSize,
		},

		&cli.IntFlag{
			Name:        "write_gopool_size",
			Usage:       "The size of the goroutine pool to write client messages",
			Value:       c.App.WriteGopoolSize,
			Destination: &c.App.WriteGopoolSize,
		},

		&cli.BoolFlag{
			Name:        "netpoll_enabled",
			Usage:       "Whether to use net polling (epoll, kqueue) to read data or not",
			Value:       c.App.NetpollEnabled,
			Destination: &c.App.NetpollEnabled,
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
			Hidden:      true,
		},

		&cli.IntFlag{
			Name:        "pong_timeout",
			Usage:       `How long to wait for a pong response before disconnecting the client (in seconds). Zero means no pongs required`,
			Value:       c.App.PongTimeout,
			Destination: &c.App.PongTimeout,
		},
	})
}

// jwtCLIFlags returns CLI flags for JWT
func jwtCLIFlags(c *config.Config, jwtIdKey *string, jwtIdParam *string, jwtIdEnforce *bool) []cli.Flag {
	return withDefaults(jwtCategoryDescription, []cli.Flag{
		&cli.StringFlag{
			Name:        "jwt_id_key",
			Destination: jwtIdKey,
			Usage:       "[Depracated]",
			Hidden:      true,
		},

		&cli.StringFlag{
			Name:        "jwt_secret",
			Usage:       "The encryption key used to verify JWT tokens",
			Destination: &c.JWT.Secret,
		},

		&cli.StringFlag{
			Name:        "jwt_id_param",
			Destination: jwtIdParam,
			Usage:       "[Deprecated]",
			Hidden:      true,
		},

		&cli.StringFlag{
			Name:        "jwt_param",
			Usage:       "The name of a query string param or an HTTP header carrying a token",
			Value:       c.JWT.Param,
			Destination: &c.JWT.Param,
		},

		&cli.BoolFlag{
			Name:        "jwt_id_enforce",
			Usage:       "[Deprecated]",
			Destination: jwtIdEnforce,
			Hidden:      true,
		},

		&cli.BoolFlag{
			Name:        "enforce_jwt",
			Usage:       "Whether to enforce token presence for all connections",
			Destination: &c.JWT.Force,
		},
	})
}

// signedStreamsCLIFlags returns misc CLI flags
func signedStreamsCLIFlags(c *config.Config, turboRailsKey *string, cableReadyKey *string, turboRailsClearText *bool, cableReadyCleartext *bool) []cli.Flag {
	return withDefaults(signedStreamsCategoryDescription, []cli.Flag{
		&cli.StringFlag{
			Name:        "streams_secret",
			Usage:       "Secret you use to sign stream names",
			Destination: &c.Streams.Secret,
		},

		&cli.BoolFlag{
			Name:        "public_streams",
			Usage:       "Enable public (unsigned) streams",
			Destination: &c.Streams.Public,
		},

		&cli.BoolFlag{
			Name:        "streams_whisper",
			Usage:       "Enable whispering for signed pub/sub streams",
			Destination: &c.Streams.Whisper,
		},

		&cli.BoolFlag{
			Name:        "turbo_streams",
			Usage:       "Enable Turbo Streams support",
			Destination: &c.Streams.Turbo,
		},

		&cli.BoolFlag{
			Name:        "cable_ready",
			Usage:       "Enable Cable Ready support",
			Destination: &c.Streams.CableReady,
		},

		&cli.StringFlag{
			Name:        "turbo_rails_key",
			Usage:       "[Deprecated]",
			Destination: turboRailsKey,
			Hidden:      true,
		},

		&cli.StringFlag{
			Name:        "turbo_streams_secret",
			Usage:       "A custom secret to verify Turbo Streams",
			Destination: &c.Streams.TurboSecret,
		},

		&cli.BoolFlag{
			Name:        "turbo_rails_cleartext",
			Usage:       "[DEPRECATED] Enable Turbo Streams fastlane without stream names signing",
			Destination: turboRailsClearText,
			Hidden:      true,
		},

		&cli.StringFlag{
			Name:        "cable_ready_key",
			Usage:       "[Deprecated]",
			Destination: cableReadyKey,
			Hidden:      true,
		},

		&cli.StringFlag{
			Name:        "cable_ready_secret",
			Usage:       "A custom secret to verify CableReady streams",
			Destination: &c.Streams.CableReadySecret,
		},

		&cli.BoolFlag{
			Name:        "cable_ready_cleartext",
			Usage:       "[DEPRECATED] Enable Cable Ready fastlane without stream names signing",
			Destination: cableReadyCleartext,
			Hidden:      true,
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
			Hidden:      true,
		},
		&cli.StringFlag{
			Name:        "statsd_tags_format",
			Usage:       `One of "datadog", "influxdb", or "graphite"`,
			Value:       c.Metrics.Statsd.TagFormat,
			Destination: &c.Metrics.Statsd.TagFormat,
		},
	})
}

// sseCLIFlags returns CLI flags for SSE
func sseCLIFlags(c *config.Config) []cli.Flag {
	return withDefaults(sseCategoryDescription, []cli.Flag{
		&cli.BoolFlag{
			Name:        "sse",
			Usage:       "Enable SSE endpoint",
			Value:       c.SSE.Enabled,
			Destination: &c.SSE.Enabled,
		},
		&cli.StringFlag{
			Name:        "sse_path",
			Usage:       "SSE endpoint path",
			Value:       c.SSE.Path,
			Destination: &c.SSE.Path,
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

// graphqlCLIFlags returns GraphQL specific flags
func graphqlCLIFlags(c *config.Config) []cli.Flag {
	return withDefaults(graphqlCategoryDescription, []cli.Flag{
		&cli.StringFlag{
			Name:        "graphql_path",
			Usage:       "Enable GraphQL proxy and mount at the specified path",
			Destination: &c.GraphQL.Path,
		},
		&cli.StringFlag{
			Name:        "graphql_channel",
			Usage:       "GraphQL Ruby channel class name",
			Value:       c.GraphQL.Channel,
			Destination: &c.GraphQL.Channel,
		},
		&cli.StringFlag{
			Name:        "graphql_action",
			Usage:       "GraphQL Ruby channel action (method) name",
			Value:       c.GraphQL.Action,
			Destination: &c.GraphQL.Action,
		},
		&cli.IntFlag{
			Name:        "graphql_idle_timeout",
			Usage:       `Defines for how long to wait for a client to send "connection_init" message (in seconds)`,
			Value:       c.GraphQL.IdleTimeout,
			Destination: &c.GraphQL.IdleTimeout,
		},
		&cli.StringFlag{
			Name:        "apollo_path",
			Hidden:      true,
			Destination: &c.LegacyGraphQL.Path,
		},
		&cli.StringFlag{
			Name:        "apollo_channel",
			Hidden:      true,
			Destination: &c.LegacyGraphQL.Channel,
		},
		&cli.StringFlag{
			Name:        "apollo_action",
			Hidden:      true,
			Destination: &c.LegacyGraphQL.Action,
		},
	})
}

func ocppCLIFlags(c *config.Config) []cli.Flag {
	return withDefaults(ocppCategoryDescription, []cli.Flag{
		&cli.StringFlag{
			Name:        "ocpp_path",
			Usage:       "WebSocket endpoint path prefix to accept OCPP connections",
			Destination: &c.OCPP.Path,
		},
		&cli.StringFlag{
			Name:        "ocpp_channel",
			Usage:       "Action Cable channel class name to use at the Ruby side to manage OCPP connections",
			Value:       c.OCPP.ChannelName,
			Destination: &c.OCPP.ChannelName,
		},
		&cli.IntFlag{
			Name:        "ocpp_hearbeat_interval",
			Usage:       "Default heartbeat interval in seconds",
			Value:       c.OCPP.HeartbeatInterval,
			Destination: &c.OCPP.HeartbeatInterval,
		},
		&cli.BoolFlag{
			Name: "ocpp_granular_actions",
			Usage: `When enabled, each OCPP command is translated into its own Action Cable action (e.g, "BootNotification" -> "#boot_notification").
				When disabled, all commands are sent to the #receive method`,
			Value:       c.OCPP.GranularActions,
			Destination: &c.OCPP.GranularActions,
		},
	})
}

// longPollingCLIFlags returns CLI flags for long polling settings
func longPollingCLIFlags(c *config.Config) []cli.Flag {
	return withDefaults(longPollingCategoryDescription, []cli.Flag{
		&cli.BoolFlag{
			Name:        "poll",
			Usage:       "Enable long polling support",
			Value:       c.LongPolling.Enabled,
			Destination: &c.LongPolling.Enabled,
		},
		&cli.IntFlag{
			Name:        "poll_interval",
			Usage:       "Long polling interval (in seconds)",
			Value:       c.LongPolling.PollInterval,
			Destination: &c.LongPolling.PollInterval,
		},
		&cli.IntFlag{
			Name:        "poll_flush_interval",
			Usage:       "Long polling flush interval (in milliseconds) defines for how long to buffer server-to-client messages before flushing them to the client",
			Value:       c.LongPolling.FlushInterval,
			Destination: &c.LongPolling.FlushInterval,
		},
		&cli.StringFlag{
			Name:        "poll_path",
			Usage:       "Long polling endpoint path",
			Value:       c.LongPolling.Path,
			Destination: &c.LongPolling.Path,
		},
		&cli.IntFlag{
			Name:        "poll_max_request_size",
			Usage:       "Long polling maximum request body size (in bytes)",
			Value:       c.LongPolling.MaxBodySize,
			Destination: &c.LongPolling.MaxBodySize,
		},
		&cli.IntFlag{
			Name:        "poll_keepalive_timeout",
			Usage:       "Long polling keepalive timeout (in seconds) defines for how long to wait between poll requests before marking client as disconnected.",
			Value:       c.LongPolling.KeepaliveTimeout,
			Destination: &c.LongPolling.KeepaliveTimeout,
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
