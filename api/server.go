package api

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/anycable/anycable-go/broadcast"
	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/server"
	"github.com/anycable/anycable-go/utils"
)

// Server manages the HTTP API server
type Server struct {
	server *server.HTTPServer
	conf   *Config

	broadcastHandler broadcast.Handler
	broker           broker.Broker

	authenticator *Authenticator

	enableCORS   bool
	allowedHosts []string

	log    *slog.Logger
	closed bool
	mu     sync.Mutex
}

// NewServer creates a new API server with the given configuration
func NewServer(conf *Config, brk broker.Broker, handler broadcast.Handler, l *slog.Logger) (*Server, error) {
	// Derive secret if needed
	if err := conf.DeriveSecret(); err != nil {
		return nil, err
	}

	auth, err := NewAuthenticator(conf.Secret)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticator: %w", err)
	}

	s := &Server{
		conf:             conf,
		broadcastHandler: handler,
		broker:           brk,
		authenticator:    auth,
		log:              l.With("context", "api"),
	}

	if conf.AddCORSHeaders {
		s.enableCORS = true
		if conf.CORSHosts != "" {
			s.allowedHosts = strings.Split(conf.CORSHosts, ",")
		} else {
			s.allowedHosts = []string{}
		}
	}

	return s, nil
}

// Start initializes the HTTP server and registers handlers
func (s *Server) Start() error {
	var srv *server.HTTPServer
	var err error

	if s.conf.Host != "" && s.conf.Host != server.Host {
		srv, err = server.NewServer(s.conf.Host, strconv.Itoa(s.conf.Port), server.SSL, server.MaxConn)
	} else {
		srv, err = server.ForPort(strconv.Itoa(s.conf.Port))
	}

	if err != nil {
		return fmt.Errorf("failed to initialize API server: %w", err)
	}

	s.server = srv

	publishPath := s.conf.Path + "/publish"
	s.server.SetupHandler(publishPath, http.HandlerFunc(s.PublishHandler))

	presencePath := s.conf.Path + "/presence/*"
	s.server.SetupHandler(presencePath, http.HandlerFunc(s.PresenceHandler))

	return nil
}

// Run starts the HTTP server (blocking)
func (s *Server) Run() error {
	if s.server == nil {
		return fmt.Errorf("server not initialized, call Start() first")
	}

	var verifiedVia string
	if s.authenticator.IsEnabled() {
		verifiedVia = "authorization required"
	} else {
		verifiedVia = "no authorization"
	}

	if s.enableCORS {
		verifiedVia += ", CORS enabled"
	}

	s.log.Info(fmt.Sprintf("Handle API requests at %s%s (%s)", s.server.Address(), s.conf.Path, verifiedVia))

	if err := s.server.StartAndAnnounce("API server"); err != nil {
		if !s.server.Stopped() {
			return fmt.Errorf("API HTTP server at %s stopped: %v", s.server.Address(), err)
		}
	}

	return nil
}

// Shutdown gracefully stops the API server
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	if s.server != nil {
		return s.server.Shutdown(ctx)
	}

	return nil
}

// Address returns the server address (for testing/logging)
func (s *Server) Address() string {
	if s.server != nil {
		return s.server.Address()
	}
	return ""
}

// Authenticator returns the server's authenticator (for testing)
func (s *Server) Authenticator() *Authenticator {
	return s.authenticator
}

// PublishHandler handles POST requests to publish broadcast messages
func (s *Server) PublishHandler(w http.ResponseWriter, r *http.Request) {
	if s.enableCORS {
		server.WriteCORSHeaders(w, r, s.allowedHosts)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	if r.Method != http.MethodPost {
		s.log.Debug("invalid request method", "method", r.Method)
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	if !s.authenticator.Authenticate(r) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.log.Error("failed to read request body", "error", err)
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	if err = s.broadcastHandler.HandleBroadcast(body); err == nil {
		w.WriteHeader(http.StatusCreated)
	} else {
		s.log.Error("failed to handle broadcast", "error", err)
		w.WriteHeader(http.StatusNotImplemented)
	}
}

var channelUsersPathPattern = regexp.MustCompile(`/presence/([^/]+)/users$`)

// PresenseHandler handles GET requests to get the stream's presence info
func (s *Server) PresenceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.log.Debug("invalid request method", "method", r.Method)
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	if !s.authenticator.Authenticate(r) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	matches := channelUsersPathPattern.FindStringSubmatch(r.URL.Path)

	if len(matches) < 2 {
		s.log.Debug("invalid path for get users", "path", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if !s.broker.SupportsPresence() {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	stream := matches[1]

	opts := broker.NewPresenceInfoOptions()
	info, err := s.broker.PresenceInfo(stream, broker.WithPresenceInfoOptions(opts))

	if err != nil {
		s.log.Debug("failed to get presence info", "strean", stream, "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(utils.ToJSON(info)) // nolint:errcheck
}
