package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joomcode/errorx"
	"golang.org/x/net/netutil"
)

// HTTPServer is wrapper over http.Server
type HTTPServer struct {
	server   *http.Server
	addr     string
	secured  bool
	shutdown bool
	started  bool
	maxConn  int
	mu       sync.Mutex
	log      *slog.Logger

	shutdownCtx context.Context
	shutdownFn  context.CancelFunc

	mux *chi.Mux
}

var (
	allServers   map[string]*HTTPServer = make(map[string]*HTTPServer)
	allServersMu sync.Mutex
	// Host is a default bind address for HTTP servers
	Host string = "localhost"
	// SSL is a default configuration for HTTP servers
	SSL *SSLConfig
	// MaxConn is a default configuration for maximum connections
	MaxConn int
	// Default logger
	Logger *slog.Logger
)

// ForPort creates new or returns the existing server for the specified port
func ForPort(port string) (*HTTPServer, error) {
	allServersMu.Lock()
	defer allServersMu.Unlock()

	if _, ok := allServers[port]; !ok {
		server, err := NewServer(Host, port, SSL, MaxConn)
		if err != nil {
			return nil, err
		}
		allServers[port] = server
	}

	return allServers[port], nil
}

// NewServer builds HTTPServer from config params
func NewServer(host string, port string, ssl *SSLConfig, maxConn int) (*HTTPServer, error) {
	router := chi.NewRouter()
	addr := net.JoinHostPort(host, port)

	server := &http.Server{Addr: addr, Handler: router, ReadHeaderTimeout: 5 * time.Second}

	secured := (ssl != nil) && ssl.Available()

	if secured {
		cer, err := tls.LoadX509KeyPair(ssl.CertPath, ssl.KeyPath)
		if err != nil {
			return nil, errorx.Decorate(err, "failed to load SSL certificate")
		}

		server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cer}, MinVersion: tls.VersionTLS12}
	}

	shutdownCtx, shutdownFn := context.WithCancel(context.Background())

	return &HTTPServer{
		server:      server,
		addr:        addr,
		mux:         router,
		secured:     secured,
		shutdown:    false,
		started:     false,
		shutdownCtx: shutdownCtx,
		shutdownFn:  shutdownFn,
		maxConn:     maxConn,
		log:         Logger.With("context", "http"),
	}, nil
}

// Start server
func (s *HTTPServer) Start() error {
	s.mu.Lock()
	if s.Running() {
		s.mu.Unlock()
		return nil
	}

	s.started = true
	s.mu.Unlock()

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	if s.maxConn > 0 {
		ln = netutil.LimitListener(ln, s.maxConn)
	}

	if s.secured {
		return s.server.ServeTLS(ln, "", "")
	}

	return s.server.Serve(ln)
}

// StartAndAnnounce prints server info and starts server
func (s *HTTPServer) StartAndAnnounce(name string) error {
	s.mu.Lock()
	if s.Running() {
		s.mu.Unlock()
		s.log.Debug("HTTP server has been already started", "name", name, "addr", s.Address())
		return nil
	}

	s.log.Debug("starting HTTP server", "name", name, "addr", s.Address())
	s.mu.Unlock()

	return s.Start()
}

// Running returns true if server has been started
func (s *HTTPServer) Running() bool {
	return s.started
}

// SetupHandler adds new handler to mux
func (s *HTTPServer) SetupHandler(path string, handler http.Handler) {
	s.mux.Handle(path, handler)
}

// Shutdown shuts down server gracefully.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if s.shutdown {
		s.mu.Unlock()
		return nil
	}
	s.shutdown = true
	s.mu.Unlock()

	s.shutdownFn()

	return s.server.Shutdown(ctx)
}

// ShutdownCtx returns context for graceful shutdown.
// It must be used by HTTP handlers to termniates long-running requests (SSE, long-polling).
func (s *HTTPServer) ShutdownCtx() context.Context {
	return s.shutdownCtx
}

// Stopped return true iff server has been stopped by user
func (s *HTTPServer) Stopped() bool {
	s.mu.Lock()
	val := s.shutdown
	s.mu.Unlock()
	return val
}

// Address returns server scheme://host:port
func (s *HTTPServer) Address() string {
	var scheme string

	if s.secured {
		scheme = "https://"
	} else {
		scheme = "http://"
	}

	return fmt.Sprintf("%s%s", scheme, s.addr)
}
