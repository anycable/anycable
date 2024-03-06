package broadcast

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

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
	Port int
	// Path for HTTP broadast
	Path string
	// Secret token to authorize requests
	Secret string
	// SecretBase is a secret used to generate a token if none provided
	SecretBase string
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

// HTTPBroadcaster represents HTTP broadcaster
type HTTPBroadcaster struct {
	port       int
	path       string
	conf       *HTTPConfig
	authHeader string
	server     *server.HTTPServer
	node       Handler
	log        *slog.Logger
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
