package cli

import (
	_ "embed"

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
	paths       string
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
	fs.StringVar(&paths, "path", "/cable", "")
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
	fs.IntVar(&defaults.RPC.MaxRecvSize, "rpc_max_call_recv_size", 0, "")
	fs.IntVar(&defaults.RPC.MaxSendSize, "rpc_max_call_send_size", 0, "")
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

	fs.IntVar(&defaults.App.PingInterval, "ping_interval", 3, "")
	fs.StringVar(&defaults.App.PingTimestampPrecision, "ping_timestamp_precision", "s", "")
	fs.IntVar(&defaults.App.StatsRefreshInterval, "stats_refresh_interval", 5, "")
	fs.IntVar(&defaults.App.HubGopoolSize, "hub_gopool_size", 16, "")

	fs.StringVar(&defaults.JWT.Secret, "jwt_id_key", "", "")
	fs.StringVar(&defaults.JWT.Param, "jwt_id_param", "jid", "")
	fs.BoolVar(&defaults.JWT.Force, "jwt_id_enforce", false, "")

	fs.StringVar(&defaults.Rails.TurboRailsKey, "turbo_rails_key", "", "")
	fs.StringVar(&defaults.Rails.CableReadyKey, "cable_ready_key", "", "")

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

//go:embed usage.txt
var usage string

// PrintHelp prints CLI usage instructions to STDOUT
func PrintHelp() {
	fmt.Print(usage)
}

func prepareComplexDefaults() {
	defaults.Headers = parseHeaders(headers)
	defaults.Path = strings.Split(paths, " ")

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
