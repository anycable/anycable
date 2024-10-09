package broadcast

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/utils"
	"github.com/joomcode/errorx"
)

const (
	defaultHTTPPath    = "/_broadcast"
	broadcastKeyPhrase = "broadcast-cable"
)

// HTTPConfig contains HTTP pubsub adapter configuration
type HTTPConfig struct {
	// Port to listen on
	Port int `toml:"port"`
	// Path for HTTP broadast
	Path string `toml:"path"`
	// Secret token to authorize requests
	Secret string `toml:"secret"`
	// SecretBase is a secret used to generate a token if none provided
	SecretBase string
	// AddCORSHeaders enables adding CORS headers (so you can perform broadcast requests from the browser)
	// (We mostly need it for Stackblitz)
	AddCORSHeaders bool `toml:"cors_headers"`
	// CORSHosts contains a list of hostnames for CORS (comma-separated)
	CORSHosts string `toml:"cors_hosts"`
}

// NewHTTPConfig builds a new config for HTTP pub/sub
func NewHTTPConfig() HTTPConfig {
	return HTTPConfig{
		Path: defaultHTTPPath,
	}
}

func (c *HTTPConfig) IsSecured() bool {
	return c.Secret != "" || c.SecretBase != ""
}

func (c HTTPConfig) ToToml() string {
	var result strings.Builder

	result.WriteString("# HTTP server port (can be the same as the main server port)\n")
	result.WriteString(fmt.Sprintf("port = %d\n", c.Port))

	result.WriteString("# HTTP endpoint path for broadcasts\n")
	result.WriteString(fmt.Sprintf("path = \"%s\"\n", c.Path))

	result.WriteString("# Secret token to authenticate broadcasting requests\n")
	if c.Secret != "" {
		result.WriteString(fmt.Sprintf("secret = \"%s\"\n", c.Secret))
	} else {
		result.WriteString("# secret = \"\"\n")
	}

	result.WriteString("# Enable CORS headers (allowed origins are used as allowed hosts)\n")
	if c.AddCORSHeaders {
		result.WriteString("cors_headers = true\n")
	} else {
		result.WriteString("# cors_headers = false\n")
	}

	result.WriteString("\n")

	return result.String()
}

// HTTPBroadcaster represents HTTP broadcaster
type HTTPBroadcaster struct {
	port         int
	path         string
	conf         *HTTPConfig
	authHeader   string
	enableCORS   bool
	allowedHosts []string
	server       *server.HTTPServer
	node         Handler
	log          *slog.Logger
}

var _ Broadcaster = (*HTTPBroadcaster)(nil)

// NewHTTPBroadcaster builds a new HTTPSubscriber struct
func NewHTTPBroadcaster(node Handler, config *HTTPConfig, l *slog.Logger) *HTTPBroadcaster {
	return &HTTPBroadcaster{
		node: node,
		log:  l.With("context", "broadcast").With("provider", "http"),
		port: config.Port,
		path: config.Path,
		conf: config,
	}
}

func (HTTPBroadcaster) IsFanout() bool {
	return false
}

// Prepare configures the broadcaster to make it ready to accept requests
// (i.e., calculates the authentication token, etc.)
func (s *HTTPBroadcaster) Prepare() error {
	authHeader := ""

	if s.conf.Secret == "" && s.conf.SecretBase != "" {
		secret, err := utils.NewMessageVerifier(s.conf.SecretBase).Sign([]byte(broadcastKeyPhrase))

		if err != nil {
			err = errorx.Decorate(err, "failed to auto-generate authentication key for HTTP broadcaster")
			return err
		}

		s.log.Info("auto-generated authorization secret from the application secret")
		s.conf.Secret = string(secret)
	}

	if s.conf.Secret != "" {
		authHeader = fmt.Sprintf("Bearer %s", s.conf.Secret)
	}

	s.authHeader = authHeader

	if s.conf.AddCORSHeaders {
		s.enableCORS = true
		if s.conf.CORSHosts != "" {
			s.allowedHosts = strings.Split(s.conf.CORSHosts, ",")
		} else {
			s.allowedHosts = []string{}
		}
	}

	return nil
}

// Start creates an HTTP server or attaches a handler to the existing one
func (s *HTTPBroadcaster) Start(done chan (error)) error {
	server, err := server.ForPort(strconv.Itoa(s.port))

	if err != nil {
		return err
	}

	err = s.Prepare()
	if err != nil {
		return err
	}

	s.server = server
	s.server.SetupHandler(s.path, http.HandlerFunc(s.Handler))

	var verifiedVia string

	if s.authHeader != "" {
		verifiedVia = "authorization required"
	} else {
		verifiedVia = "no authorization"
	}

	if s.enableCORS {
		verifiedVia += ", CORS enabled"
	}

	s.log.Info(fmt.Sprintf("Accept broadcast requests at %s%s (%s)", s.server.Address(), s.path, verifiedVia))

	go func() {
		if err := s.server.StartAndAnnounce("broadcasting HTTP server"); err != nil {
			if !s.server.Stopped() {
				done <- fmt.Errorf("broadcasting HTTP server at %s stopped: %v", s.server.Address(), err)
			}
		}
	}()

	return nil
}

// Shutdown stops the HTTP server
func (s *HTTPBroadcaster) Shutdown(ctx context.Context) error {
	if s.server != nil {
		s.server.Shutdown(ctx) //nolint:errcheck
	}

	return nil
}

// Handler processes HTTP requests
func (s *HTTPBroadcaster) Handler(w http.ResponseWriter, r *http.Request) {
	if s.enableCORS {
		// Write CORS headers
		server.WriteCORSHeaders(w, r, s.allowedHosts)

		// Respond to preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	if r.Method != "POST" {
		s.log.Debug("invalid request method", "method", r.Method)
		w.WriteHeader(422)
		return
	}

	if s.authHeader != "" {
		if r.Header.Get("Authorization") != s.authHeader {
			w.WriteHeader(401)
			return
		}
	}

	body, err := io.ReadAll(r.Body)

	if err != nil {
		s.log.Error("failed to read request body")
		w.WriteHeader(422)
		return
	}

	s.node.HandleBroadcast(body)

	w.WriteHeader(201)
}
