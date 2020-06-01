package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/apex/log"
)

// HTTPServer is wrapper over http.Server
type HTTPServer struct {
	server   *http.Server
	addr     string
	secured  bool
	shutdown bool
	started  bool
	mu       sync.Mutex
	log      *log.Entry

	Mux *http.ServeMux
}

var (
	allServers map[string]*HTTPServer = make(map[string]*HTTPServer)
	// Host is a default bind address for HTTP servers
	Host string = "localhost"
	// SSL is a default configuration for HTTP servers
	SSL *SSLConfig
)

// ForPort creates new or returns the existing server for the specified port
func ForPort(port string) (*HTTPServer, error) {
	if _, ok := allServers[port]; !ok {
		server, err := NewServer(Host, port, SSL)
		if err != nil {
			return nil, err
		}
		allServers[port] = server
	}

	return allServers[port], nil
}

// NewServer builds HTTPServer from config params
func NewServer(host string, port string, ssl *SSLConfig) (*HTTPServer, error) {
	mux := http.NewServeMux()
	addr := net.JoinHostPort(host, port)

	server := &http.Server{Addr: addr, Handler: mux}

	secured := (ssl != nil) && ssl.Available()

	if secured {
		cer, err := tls.LoadX509KeyPair(ssl.CertPath, ssl.KeyPath)
		if err != nil {
			msg := fmt.Sprintf("Failed to load SSL certificate: %s.", err)
			return nil, errors.New(msg)
		}

		server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cer}}
	}

	return &HTTPServer{
		server:   server,
		addr:     addr,
		Mux:      mux,
		secured:  secured,
		shutdown: false,
		started:  false,
		log:      log.WithField("context", "http"),
	}, nil
}

// Start server
func (s *HTTPServer) Start() error {
	if s.Running() {
		return nil
	}

	s.started = true

	if s.secured {
		s.log.Infof("Starting HTTPS server at %v", s.addr)
		return s.server.ListenAndServeTLS("", "")
	}

	s.log.Infof("Starting HTTP server at %v", s.addr)
	return s.server.ListenAndServe()
}

// Running returns true if server has been started
func (s *HTTPServer) Running() bool {
	return s.started
}

// Stop shuts down server gracefully.
func (s *HTTPServer) Stop() error {
	s.mu.Lock()
	if s.shutdown {
		return nil
	}
	s.shutdown = true
	s.mu.Unlock()

	return s.server.Shutdown(context.Background())
}

// Stopped return true iff server has been stopped by user
func (s *HTTPServer) Stopped() bool {
	s.mu.Lock()
	val := s.shutdown
	s.mu.Unlock()
	return val
}

// Address returns server host:port
func (s *HTTPServer) Address() string {
	return s.addr
}
