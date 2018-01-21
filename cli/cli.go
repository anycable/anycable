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
	fs.StringVar(&defaults.RPCHost, "rpc_host", "0.0.0.0:50051", "")
	fs.StringVar(&defaults.RedisURL, "redis_url", redisDefault, "")
	fs.StringVar(&defaults.RedisChannel, "redis_channel", "__anycable__", "")
	fs.StringVar(&defaults.Host, "host", "0.0.0.0", "")
	fs.IntVar(&defaults.Port, "port", portDefault, "")
	fs.StringVar(&defaults.Path, "path", "/cable", "")
	fs.IntVar(&defaults.DisconnectRate, "disconnect_rate", 100, "")
	fs.StringVar(&defaults.SSL.CertPath, "ssl_cert", "", "")
	fs.StringVar(&defaults.SSL.KeyPath, "ssl_key", "", "")
	fs.StringVar(&defaults.LogLevel, "log_level", "info", "")
	fs.StringVar(&defaults.LogFormat, "log_format", "text", "")
	// CLI vars
	fs.BoolVar(&showVersion, "v", false, "")
	fs.BoolVar(&showHelp, "h", false, "")
	fs.StringVar(&headers, "headers", "cookie", "")
	fs.BoolVar(&debugMode, "debug", false, "")
}

// GetConfig returns CLI configuration
func GetConfig() config.Config {
	ensureParsed()
	return defaults
}

// ShowVersion returns true if -v flag was provided
func ShowVersion() bool {
	ensureParsed()
	return showVersion
}

// ShowHelp returns true if -h flag was provided
func ShowHelp() bool {
	ensureParsed()
	return showHelp
}

// DebugMode returns true if -debug flag is provided
func DebugMode() bool {
	ensureParsed()
	return debugMode
}

const usage = `AnyCable-Go, The WebSocket server for anycable.io

Usage:
	Usage: anycable-go [options]

The flags are:

  --host              Server host, default: localhost, env: ANYCABLE_HOST
  --port              Server port, default: 8080, env: ANYCABLE_PORT, PORT
  --path              WebSocket endpoint path, default: /cable, env: ANYCABLE_PATH

  --ssl_cert          SSL certificate path, env: ANYCABLE_SSL_CERT
  --ssl_key           SSL private key path, env: ANYCABLE_SSL_KEY

  --redis_url         Redis url, default: redis://localhost:6379/5, env: ANYCABLE_REDIS_URL, REDIS_URL
  --redis_channel     Redis channel for broadcasts, default: __anycable__, env: ANYCABLE_REDIS_CHANNEL

  --rpc_host          RPC service address, default: 0.0.0.0:50051, env: ANYCABLE_RPC_HOST
  --headers           List of headers to proxy to RPC, default: cookie, env: ANYCABLE_HEADERS
  --disconnect_rate   Max number of Disconnect calls per second, default: 100, env: ANYCABLE_DISCONNECT_RATE

  --log_level         Set logging level (debug/info/warn/error/fatal), default: info, env: ANYCABLE_LOG_LEVEL
  --log_format        Set logging format (text, json), default: text, env: ANYCABLE_LOG_FORMAT
  --debug             Enable debug mode (more verbose logging), default: false, env: ANYCABLE_DEBUG

  -h                  This help screen
  -v                  Show version

`

// PrintHelp prints CLI usage instructions to STDOUT
func PrintHelp() {
	fmt.Printf(usage)
}

func ensureParsed() {
	if !fs.Parsed() {
		fs.Parse(os.Args[1:])
		defaults.Headers = parseHeaders(headers)

		if debugMode {
			defaults.LogLevel = "debug"
			defaults.LogFormat = "text"
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
