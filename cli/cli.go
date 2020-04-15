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

const (
	deprecatedHost = "0.0.0.0|deprecated"
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
	fs.StringVar(&defaults.Host, "host", deprecatedHost, "")
	fs.IntVar(&defaults.Port, "port", portDefault, "")
	fs.StringVar(&defaults.Path, "path", "/cable", "")
	fs.StringVar(&defaults.HealthPath, "health-path", "/health", "")

	fs.StringVar(&defaults.SSL.CertPath, "ssl_cert", "", "")
	fs.StringVar(&defaults.SSL.KeyPath, "ssl_key", "", "")

	fs.StringVar(&defaults.RedisURL, "redis_url", redisDefault, "")
	fs.StringVar(&defaults.RedisChannel, "redis_channel", "__anycable__", "")

	fs.BoolVar(&defaults.RedisSentinelEnabled, "redis_sentinel_enabled", false, "")
	fs.StringVar(&defaults.RedisSentinels, "redis_sentinels", "", "")
	fs.StringVar(&defaults.RedisPassword, "redis_password", "", "")
	fs.StringVar(&defaults.RedisMasterName, "redis_master_name", "mymaster", "")

	fs.StringVar(&defaults.RPCHost, "rpc_host", "localhost:50051", "")
	fs.StringVar(&headers, "headers", "cookie", "")
	fs.IntVar(&defaults.DisconnectRate, "disconnect_rate", 100, "")
	fs.Int64Var(&defaults.MaxMessageSize, "max_message_size", 65536, "")

	fs.StringVar(&defaults.LogLevel, "log_level", "info", "")
	fs.StringVar(&defaults.LogFormat, "log_format", "text", "")
	fs.BoolVar(&debugMode, "debug", false, "")

	fs.BoolVar(&defaults.MetricsLog, "metrics_log", false, "")
	fs.IntVar(&defaults.MetricsLogInterval, "metrics_log_interval", 15, "")
	fs.StringVar(&defaults.MetricsLogFormatter, "metrics_log_formatter", "", "")
	fs.StringVar(&defaults.MetricsHTTP, "metrics_http", "", "")
	fs.StringVar(&defaults.MetricsHost, "metrics_host", "", "")
	fs.IntVar(&defaults.MetricsPort, "metrics_port", 0, "")

	// CLI vars
	fs.BoolVar(&showHelp, "h", false, "")
	fs.BoolVar(&showVersion, "v", false, "")

	fs.Parse(os.Args[1:])
}

// Config returns CLI configuration
func Config() config.Config {
	prepareComplexDefaults()
	return defaults
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
  --host                     Server host, default: 0.0.0.0 (deprecated, will be changed to "localhost"), env: ANYCABLE_HOST
  --port                     Server port, default: 8080, env: ANYCABLE_PORT, PORT
  --path                     WebSocket endpoint path, default: /cable, env: ANYCABLE_PATH
  --health-path              HTTP health endpoint path, default: /health, env: ANYCABLE_HEALTH_PATH

  --ssl_cert                 SSL certificate path, env: ANYCABLE_SSL_CERT
  --ssl_key                  SSL private key path, env: ANYCABLE_SSL_KEY

  --redis_url                Redis url, default: redis://localhost:6379/5, env: ANYCABLE_REDIS_URL, REDIS_URL
  --redis_channel            Redis channel for broadcasts, default: __anycable__, env: ANYCABLE_REDIS_CHANNEL

  --redis_sentinel_enabled   Enable redis sentinel support, default: false, env: ANYCABLE_REDIS_SENTINEL_ENABLED
  --redis_sentinels			 Comma separated list of sentinel hosts, env: ANYCABLE_REDIS_SENTINELS
  --redis_password			 Redis password (only used if redis_sentinel_enabled is true), env: ANYCABLE_REDIS_PASSWORD
  --redis_master_name        Redis sentinel master name, default: mymaster, env: ANYCABLE_REDIS_MASTER_NAME

  --rpc_host                 RPC service address, default: localhost:50051, env: ANYCABLE_RPC_HOST
  --headers                  List of headers to proxy to RPC, default: cookie, env: ANYCABLE_HEADERS
  --disconnect_rate          Max number of Disconnect calls per second, default: 100, env: ANYCABLE_DISCONNECT_RATE
  --max_message_size         Maximum size of a message in bytes, default: 65536, env: ANYCABLE_MAX_MESSAGE_SIZE

  --log_level                Set logging level (debug/info/warn/error/fatal), default: info, env: ANYCABLE_LOG_LEVEL
  --log_format               Set logging format (text, json), default: text, env: ANYCABLE_LOG_FORMAT
  --debug                    Enable debug mode (more verbose logging), default: false, env: ANYCABLE_DEBUG

  --metrics_log              Enable metrics logging (with info level), default: false, env: ANYCABLE_METRICS_LOG
  --metrics_log_interval     Specify how often flush metrics logs (in seconds), default: 15, env: ANYCABLE_METRICS_LOG_INTERVAL
  --metrics_log_formatter    Specify the path to custom Ruby formatter script (only supported on MacOS and Linux), default: "" (none), env: ANYCABLE_METRICS_LOG_FORMATTER
  --metrics_http             Enable HTTP metrics endpoint at the specified path, default: "" (disabled), env: ANYCABLE_METRICS_HTTP
  --metrics_host             Server host for metrics endpoint, default: the same as for main server, env: ANYCABLE_METRICS_HOST
  --metrics_port             Server port for metrics endpoint, default: the same as for main server, env: ANYCABLE_METRICS_PORT

  -h                       This help screen
  -v                       Show version

`

// PrintHelp prints CLI usage instructions to STDOUT
func PrintHelp() {
	fmt.Print(usage)
}

func prepareComplexDefaults() {
	if defaults.Host == deprecatedHost {
		fmt.Println(
			`DEPRECATION WARNING: You're using default host configuration which starts AnyCable-Go
server on all available interfaces including external ones. This is about to be changed
to loopback interface only in future versions. Please, consider switching to "localhost"
or set "0.0.0.0" explicitly in your configuration, if you want to continue with
the current behavior and supress this message.`)

		defaults.Host = "0.0.0.0"
	}
	defaults.Headers = parseHeaders(headers)

	if debugMode {
		defaults.LogLevel = "debug"
		defaults.LogFormat = "text"
	}

	if defaults.MetricsPort == 0 {
		defaults.MetricsPort = defaults.Port
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
