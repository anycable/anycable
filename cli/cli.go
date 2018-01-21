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

	// CLI vars
	fs.BoolVar(&showVersion, "v", false, "")
	fs.StringVar(&headers, "headers", "cookie", "")
	fs.BoolVar(&showHelp, "h", false, "")
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

// PrintHelp prints CLI usage instructions to STDOUT
func PrintHelp() {
	fmt.Printf("Usage: anycable-go [options]\n\n")
	fmt.Printf("Note: you can use environment variables\n\n")
	fmt.Println("-host\t\tServer host, default: localhost, env: ANYCABLE_HOST")
	fmt.Println("-port\t\tServer port, default: 8080, env: ANYCABLE_PORT, PORT")
	fmt.Println("-rpc_host\t\tRPC service address, default: 0.0.0.0:50051, env: ANYCABLE_RPC_HOST")
	fmt.Println("-redis_url\t\tRedis url, default: redis://localhost:6379/5, env: ANYCABLE_REDIS_URL, REDIS_URL")
	fmt.Println("-redis_channel\t\tRedis channel for broadcasts, default: __anycable__, env: ANYCABLE_REDIS_CHANNEL")
	fmt.Println("-path\t\t\tWebSocket endpoint path, default: /cable, env: ANYCABLE_PATH")
	fmt.Println("-headers\t\tList of headers to proxy to RPC, default: cookie, env: ANYCABLE_HEADERS")
	fmt.Println("-disconnect_rate\tMax number of Disconnect calls per second, default: 100, env: ANYCABLE_DISCONNECT_RATE")
	fmt.Println("-ssl_cert\t\tSSL certificate path, env: ANYCABLE_SSL_CERT")
	fmt.Println("-ssl_key\t\tSSL private key path, env: ANYCABLE_SSL_KEY")
	fmt.Println("-h\t\t\tThis help screen")
	fmt.Println("-v\t\t\tShow version")
}

func ensureParsed() {
	if !fs.Parsed() {
		fs.Parse(os.Args[1:])
		defaults.Headers = parseHeaders(headers)
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
