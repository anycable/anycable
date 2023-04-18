package broadcast

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/anycable/anycable-go/server"
	"github.com/apex/log"
)

const (
	defaultHTTPPort = 8090
	defaultHTTPPath = "/_broadcast"
)

// HTTPConfig contains HTTP pubsub adapter configuration
type HTTPConfig struct {
	// Port to listen on
	Port int
	// Path for HTTP broadast
	Path string
	// Secret token to authorize requests
	Secret string
}

// NewHTTPConfig builds a new config for HTTP pub/sub
func NewHTTPConfig() HTTPConfig {
	return HTTPConfig{
		Port: defaultHTTPPort,
		Path: defaultHTTPPath,
	}
}

// HTTPBroadcaster represents HTTP pub/sub
type HTTPBroadcaster struct {
	port       int
	path       string
	authHeader string
	server     *server.HTTPServer
	node       Handler
	log        *log.Entry
}

var _ Broadcaster = (*HTTPBroadcaster)(nil)

// NewHTTPBroadcaster builds a new HTTPSubscriber struct
func NewHTTPBroadcaster(node Handler, config *HTTPConfig) *HTTPBroadcaster {
	authHeader := ""

	if config.Secret != "" {
		authHeader = fmt.Sprintf("Bearer %s", config.Secret)
	}

	return &HTTPBroadcaster{
		node:       node,
		log:        log.WithFields(log.Fields{"context": "pubsub", "provider": "http"}),
		port:       config.Port,
		path:       config.Path,
		authHeader: authHeader,
	}
}

func (HTTPBroadcaster) IsFanout() bool {
	return false
}

// Start creates an HTTP server or attaches a handler to the existing one
func (s *HTTPBroadcaster) Start(done chan (error)) error {
	server, err := server.ForPort(strconv.Itoa(s.port))

	if err != nil {
		return err
	}

	s.server = server
	s.server.SetupHandler(s.path, http.HandlerFunc(s.Handler))

	s.log.Infof("Accept broadcast requests at %s%s", s.server.Address(), s.path)

	go func() {
		if err := s.server.StartAndAnnounce("Pub/Sub HTTP server"); err != nil {
			if !s.server.Stopped() {
				done <- fmt.Errorf("Pub/Sub HTTP server at %s stopped: %v", s.server.Address(), err)
			}
		}
	}()

	return nil
}

// Shutdown stops the HTTP server
func (s *HTTPBroadcaster) Shutdown() error {
	if s.server != nil {
		s.server.Shutdown() //nolint:errcheck
	}

	return nil
}

// Handler processes HTTP requests
func (s *HTTPBroadcaster) Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.log.Debugf("Invalid request method: %s", r.Method)
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
		s.log.Error("Failed to read request body")
		w.WriteHeader(422)
		return
	}

	s.node.HandlePubSub(body)

	w.WriteHeader(201)
}
