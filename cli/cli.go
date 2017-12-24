package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/namsral/flag"
)

// SSLOptions contains SSL parameters
type SSLOptions struct {
	CertPath string
	KeyPath  string
}

// Available returns true iff certificate and private keys are set
func (opts *SSLOptions) Available() bool {
	return opts.CertPath != "" && opts.KeyPath != ""
}

// Config contains main application configuration
type Config struct {
	RPCHost        string
	RedisURL       string
	RedisChannel   string
	Host           string
	Port           int
	Path           string
	Headers        []string
	SSL            SSLOptions
	DisconnectRate int
}

var (
	fs *flag.FlagSet
)

func init() {
	fs = flag.NewFlagSetWithEnvPrefix(os.Args[0], "ANYCABLE", 0)
	fs.Usage = help

	fs.String("rpc_host", "0.0.0.0:50051", "")
	fs.String("redis_url", "redis://localhost:6379/5", "")
	fs.String("redis_channel", "__anycable__", "")
	fs.String("host", "0.0.0.0", "")
	fs.Int("port", 8080, "")
	fs.String("path", "/cable", "")
	fs.String("headers", "cookie", "")
	fs.Int("disconnect_rate", 100, "")
	fs.String("ssl_cert", "", "")
	fs.String("ssl_key", "", "")

	fs.Parse(os.Args[1:])
}

// LoadConfig initializes application config form CLI options and environment variables
func LoadConfig() Config {
	defaults := Config{}
	defaults.SSL = SSLOptions{}

	// defaults.parseHeaders(*headersList)
	defaults.parseEnv()

	return defaults
}

// Prints CLI help
func help() {
	fmt.Printf("Usage: anycable-go [options]\n\n")
	fmt.Printf("Note: you can use environment variables too\n\n")
	fmt.Println("-h [host]\t\tServer host, default: localhost, env: ANYCABLE_HOST")
	fmt.Println("-p [port]\t\tServer port, default: 8080, env: ANYCABLE_PORT, PORT")
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

func (c *Config) parseHeaders(str string) {
	parts := strings.Split(str, ",")

	res := make([]string, len(parts))

	for i, v := range parts {
		res[i] = strings.ToLower(v)
	}

	c.Headers = res
}

func (c *Config) parseEnv() {
	port := os.Getenv("PORT")
	if port != "" {
		i, err := strconv.Atoi(port)
		if err == nil {
			c.Port = i
		}
	}

	redis := os.Getenv("REDIS_URL")
	if redis != "" {
		c.RedisURL = redis
	}
}
