package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/anycable/anycable-go/config"
	"github.com/namsral/flag"
)

var (
	// Config represents the CLI config loaded from options and ENV
	defaults    config.Config
	headers     string
	showVersion bool
	showHelp    bool
	debugMode   bool
	fs          *flag.FlagSet
)

func init() {
	// Configure namespaced flagSet with errorHandling set to ExitOnError (=1)
	fs = flag.NewFlagSetWithEnvPrefix(os.Args[0], "ANYCABLE", 1)
	fs.Usage = PrintHelp

	defaults = config.New()

	// Fetch
	portDefault := 8080
	port := os.Getenv("PORT")
	if port != "" {
		i, err := strconv.Atoi(port)
		if err == nil {
			portDefault = i
		}
	}

	redisDefault := "redis://localhost:6379/5"
	redis := os.Getenv("REDIS_URL")
	if redis != "" {
		redisDefault = redis
	}

	// Config vars
	fs.StringVar(&defaults.Host, "host", "localhost", "")
	fs.IntVar(&defaults.Port, "port", portDefault, "")
	fs.IntVar(&defaults.MaxConn, "max-conn", 0, "")
	fs.StringVar(&defaults.Path, "path", "/cable", "")
	fs.StringVar(&defaults.HealthPath, "health-path", "/health", "")

	fs.StringVar(&defaults.SSL.CertPath, "ssl_cert", "", "")
	fs.StringVar(&defaults.SSL.KeyPath, "ssl_key", "", "")

	fs.StringVar(&defaults.BroadcastAdapter, "broadcast_adapter", "redis", "")

	fs.StringVar(&defaults.Redis.URL, "redis_url", redisDefault, "")
	fs.StringVar(&defaults.Redis.Channel, "redis_channel", "__anycable__", "")
	fs.StringVar(&defaults.Redis.Sentinels, "redis_sentinels", "", "")
	fs.IntVar(&defaults.Redis.SentinelDiscoveryInterval, "redis_sentinel_discovery_interval", 30, "")
	fs.IntVar(&defaults.Redis.KeepalivePingInterval, "redis_keepalive_interval", 30, "")

	fs.IntVar(&defaults.HTTPPubSub.Port, "http_broadcast_port", 8090, "")
	fs.StringVar(&defaults.HTTPPubSub.Path, "http_broadcast_path", "/_broadcast", "")
	fs.StringVar(&defaults.HTTPPubSub.Secret, "http_broadcast_secret", "", "")

	fs.StringVar(&defaults.RPC.Host, "rpc_host", "localhost:50051", "")
	fs.IntVar(&defaults.RPC.Concurrency, "rpc_concurrency", 28, "")
	fs.BoolVar(&defaults.RPC.EnableTLS, "rpc_enable_tls", false, "")
	fs.StringVar(&headers, "headers", "cookie", "")

	fs.IntVar(&defaults.WS.ReadBufferSize, "read_buffer_size", 1024, "")
	fs.IntVar(&defaults.WS.WriteBufferSize, "write_buffer_size", 1024, "")
	fs.Int64Var(&defaults.WS.MaxMessageSize, "max_message_size", 65536, "")
	fs.BoolVar(&defaults.WS.EnableCompression, "enable_ws_compression", false, "")
	fs.StringVar(&defaults.WS.AllowedOrigins, "allowed_origins", "", "")

	fs.IntVar(&defaults.DisconnectQueue.Rate, "disconnect_rate", 100, "")
	fs.IntVar(&defaults.DisconnectQueue.ShutdownTimeout, "disconnect_timeout", 5, "")
	fs.BoolVar(&defaults.DisconnectorDisabled, "disable_disconnect", false, "")

	fs.StringVar(&defaults.LogLevel, "log_level", "info", "")
	fs.StringVar(&defaults.LogFormat, "log_format", "text", "")
	fs.BoolVar(&debugMode, "debug", false, "")

	fs.BoolVar(&defaults.Metrics.Log, "metrics_log", false, "")
	fs.IntVar(&defaults.Metrics.RotateInterval, "metrics_rotate_interval", 0, "")
	fs.IntVar(&defaults.Metrics.LogInterval, "metrics_log_interval", -1, "")
	fs.StringVar(&defaults.Metrics.LogFormatter, "metrics_log_formatter", "", "")
	fs.StringVar(&defaults.Metrics.HTTP, "metrics_http", "", "")
	fs.StringVar(&defaults.Metrics.Host, "metrics_host", "", "")
	fs.IntVar(&defaults.Metrics.Port, "metrics_port", 0, "")

	fs.StringVar(&defaults.Metrics.Statsd.Host, "statsd_host", "", "")
	fs.StringVar(&defaults.Metrics.Statsd.Prefix, "statsd_prefix", "anycable_go.", "")
	fs.IntVar(&defaults.Metrics.Statsd.MaxPacketSize, "statsd_max_prefix_size", 1400, "")

	fs.IntVar(&defaults.App.PingInterval, "ping_interval", 3, "")
	fs.StringVar(&defaults.App.PingTimestampPrecision, "ping_timestamp_precision", "s", "")
	fs.IntVar(&defaults.App.StatsRefreshInterval, "stats_refresh_interval", 5, "")
	fs.IntVar(&defaults.App.HubGopoolSize, "hub_gopool_size", 16, "")
	fs.IntVar(&defaults.App.ReadGopoolSize, "read_gopool_size", 1024, "")
	fs.IntVar(&defaults.App.WriteGopoolSize, "write_gopool_size", 1024, "")
	fs.BoolVar(&defaults.App.NetpollEnabled, "netpoll_enabled", true, "")

	fs.StringVar(&defaults.Apollo.Path, "apollo_path", "", "")
	fs.StringVar(&defaults.Apollo.Channel, "apollo_channel", "GraphqlChannel", "")
	fs.StringVar(&defaults.Apollo.Action, "apollo_action", "execute", "")

	fs.StringVar(&defaults.JWT.Secret, "jwt_id_key", "", "")
	fs.StringVar(&defaults.JWT.Param, "jwt_id_param", "jid", "")
	fs.BoolVar(&defaults.JWT.Force, "jwt_id_enforce", false, "")

	// CLI vars
	fs.BoolVar(&showHelp, "h", false, "")
	fs.BoolVar(&showVersion, "v", false, "")
}

// Config returns CLI configuration
func Config(args []string) (config.Config, error) {
	if err := fs.Parse(args); err != nil {
		return config.Config{}, err
	}

	prepareComplexDefaults()
	return defaults, nil
}

// ShowVersion returns true if -v flag was provided
func ShowVersion() bool {
	return showVersion
}

// ShowHelp returns true if -h flag was provided
func ShowHelp() bool {
	return showHelp
}

// DebugMode returns true if -debug flag is provided
func DebugMode() bool {
	return debugMode
}

const usage = `AnyCable-Go, The WebSocket server for https://anycable.io

USAGE
  anycable-go [options]

OPTIONS
  --host                                 Server host, default: localhost, env: ANYCABLE_HOST
  --port                                 Server port, default: 8080, env: ANYCABLE_PORT, PORT
  --max-conn                             Limit simultaneous server connections (0 – without limit), default: 0, env: ANYCABLE_MAX_CONN
  --path                                 WebSocket endpoint path, default: /cable, env: ANYCABLE_PATH
  --health-path                          HTTP health endpoint path, default: /health, env: ANYCABLE_HEALTH_PATH

  --ssl_cert                             SSL certificate path, env: ANYCABLE_SSL_CERT
  --ssl_key                              SSL private key path, env: ANYCABLE_SSL_KEY

  --broadcast_adapter                    Broadcasting adapter to use (redis or http), default: redis, env: ANYCABLE_BROADCAST_ADAPTER

  --redis_url                            Redis url, default: redis://localhost:6379/5, env: ANYCABLE_REDIS_URL, REDIS_URL
  --redis_channel                        Redis channel for broadcasts, default: __anycable__, env: ANYCABLE_REDIS_CHANNEL
  --redis_sentinels                      Comma separated list of sentinel hosts, format: 'hostname:port,..', env: ANYCABLE_REDIS_SENTINELS
  --redis_sentinel_discovery_interval    Interval to rediscover sentinels in seconds, default: 30, env: ANYCABLE_REDIS_SENTINEL_DISCOVERY_INTERVAL
  --redis_keeepalive_interval            Interval to periodically ping Redis to make sure it's alive, default: 30, env: ANYCABLE_REDIS_KEEPALIVE_INTERVAL

  --http_broadcast_port                  HTTP pub/sub server port, default: 8090, env: ANYCABLE_HTTP_BROADCAST_PORT
  --http_broadcast_path                  HTTP pub/sub endpoint path, default: /_broadcast, env: ANYCABLE_HTTP_BROADCAST_PATH
  --http_broadcast_secret                HTTP pub/sub authorization secret, default: "" (disabled), env: ANYCABLE_HTTP_BROADCAST_SECRET

  --rpc_host                             RPC service address, default: localhost:50051, env: ANYCABLE_RPC_HOST
  --rpc_concurrency                      Max number of concurrent RPC request; should be slightly less than the RPC server concurrency, default: 28, env: ANYCABLE_RPC_CONCURRENCY
  --rpc_enable_tls                       Enable client-side TLS with the RPC server, default: false, env: ANYCABLE_RPC_ENABLE_TLS
  --headers                              List of headers to proxy to RPC, default: cookie, env: ANYCABLE_HEADERS

  --disconnect_rate                      Max number of Disconnect calls per second, default: 100, env: ANYCABLE_DISCONNECT_RATE
  --disconnect_timeout                   Graceful shutdown timeouts (in seconds), default: 5, env: ANYCABLE_DISCONNECT_TIMEOUT
  --disable_disconnect                   Disable calling Disconnect callback, default: false, env: ANYCABLE_DISABLE_DISCONNECT

  --log_level                            Set logging level (debug/info/warn/error/fatal), default: info, env: ANYCABLE_LOG_LEVEL
  --log_format                           Set logging format (text, json), default: text, env: ANYCABLE_LOG_FORMAT
  --debug                                Enable debug mode (more verbose logging), default: false, env: ANYCABLE_DEBUG

  --metrics_log                          Enable metrics logging (with info level), default: false, env: ANYCABLE_METRICS_LOG
  --metrics_rotate_interval              Specify how often flush metrics to writers (logs, statsd) (in seconds), default: 15, env: ANYCABLE_METRICS_ROTATE_INTERVAL
  --metrics_log_interval                 DEPRECATED. Specify how often flush metrics logs (in seconds), default: 15, env: ANYCABLE_METRICS_LOG_INTERVAL
  --metrics_log_formatter                Specify the path to custom Ruby formatter script (only supported on MacOS and Linux), default: "" (none), env: ANYCABLE_METRICS_LOG_FORMATTER
  --metrics_http                         Enable HTTP metrics endpoint at the specified path, default: "" (disabled), env: ANYCABLE_METRICS_HTTP
  --metrics_host                         Server host for metrics endpoint, default: the same as for main server, env: ANYCABLE_METRICS_HOST
  --metrics_port                         Server port for metrics endpoint, default: the same as for main server, env: ANYCABLE_METRICS_PORT

  --statsd_host                          Server host for metrics sent to statsd server in the format <host>:<port>, default: "", env: ANYCABLE_STATSD_HOST
  --statsd_prefix                        Statsd metrics prefix, default: "anycable_go.", env: ANYCABLE_STATSD_PREFIX
  --statsd_max_packet_size               Statsd client maximum UDP packet size, default: 1400, env: ANYCABLE_STATSD_MAX_PACKET_SIZE

  --read_buffer_size                     WebSocket connection read buffer size, default: 1024, env: ANYCABLE_READ_BUFFER_SIZE
  --write_buffer_size                    WebSocket connection write buffer size, default: 1024, env: ANYCABLE_WRITE_BUFFER_SIZE
  --max_message_size                     Maximum size of a message in bytes, default: 65536, env: ANYCABLE_MAX_MESSAGE_SIZE
  --enable_ws_compression                Enable experimental WebSocket per message compression, default: false, env: ANYCABLE_ENABLE_WS_COMPRESSION
  --hub_gopool_size                      The size of the goroutines pool to broadcast messages, default: 16, env: ANYCABLE_HUB_GOPOOL_SIZE
  --allowed_origins                      Accept requests only from specified origins, e.g., "www.example.com,*example.io". No check is performed if empty, default: "", env: ANYCABLE_ALLOWED_ORIGINS

  --netpoll_enabled                      Whether to use net polling (epoll, kqueue) to read data or not, default: true, env: ANYCABLE_NETPOLL_ENABLED
  --read_gopool_size                     The size of the goroutine pool to read client messages, default: 1024, env: ANYCABLE_READ_GOPOOL_SIZE
  --write_gopool_size                    The size of the goroutine pool to write client messages, default: 1024, env: ANYCABLE_WRITE_GOPOOL_SIZE

  --ping_interval                        Action Cable ping interval (in seconds), default: 3, env: ANYCABLE_PING_INTERVAL
  --ping_timestamp_precision             Precision for timestamps in ping messages (s, ms, ns), default: s, env: ANYCABLE_PING_TIMESTAMP_PRECISION
  --stats_refresh_interval               How often to refresh the server stats (in seconds), default: 5, env: ANYCABLE_STATS_REFRESH_INTERVAL

  --apollo_path                          Enable Apollo GraphQL proxy and mount at the specified path, default: "" (disabled), env: ANYCABLE_APOLLO_PATH
  --apollo_channel                       GraphQL Ruby channel class name, default: "GraphqlChannel", env: ANYCABLE_APOLLO_CHANNEL
  --apollo_action                        GraphQL Ruby channel action name, default: "execute", env: ANYCABLE_APOLLO_ACTION

  --jwt_id_key                           The encryption key used to verify JWT tokens, default: "" (disabled), env: ANYCABLE_JWT_ID_KEY
  --jwt_id_param                         The name of a query string param or an HTTP header carrying a token, default: "jid" ("X-JID"), env: ANYCABLE_JWT_ID_PARAM
  --jwt_id_enforce                       Whether to enforce token presence for all connections, default: false, env: ANYCABLE_JWT_ID_ENFORCE

  -h                       This help screen
  -v                       Show version

`

// PrintHelp prints CLI usage instructions to STDOUT
func PrintHelp() {
	fmt.Print(usage)
}

func prepareComplexDefaults() {
	defaults.Headers = parseHeaders(headers)

	if debugMode {
		defaults.LogLevel = "debug"
		defaults.LogFormat = "text"
	}

	if defaults.Metrics.Port == 0 {
		defaults.Metrics.Port = defaults.Port
	}

	if defaults.Metrics.LogInterval > 0 {
		fmt.Println(`DEPRECATION WARNING: metrics_log_interval option is deprecated
and will be deleted in the next major release of anycable-go.
Use metrics_rotate_interval instead.`)

		if defaults.Metrics.RotateInterval == 0 {
			defaults.Metrics.RotateInterval = defaults.Metrics.LogInterval
		}
	}
}

// parseHeaders returns a headers list with the values from
// a comma-separated string list
func parseHeaders(str string) []string {
	parts := strings.Split(str, ",")

	res := make([]string, len(parts))

	for i, v := range parts {
		res[i] = strings.ToLower(v)
	}

	return res
}
