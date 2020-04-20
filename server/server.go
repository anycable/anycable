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
	mu       sync.Mutex
	log      *log.Entry

	Mux *http.ServeMux
}

// NewServer builds HTTPServer from config params
func NewServer(host string, port string, ssl *SSLConfig) (*HTTPServer, error) {
	mux := http.NewServeMux()
	addr := net.JoinHostPort(host, port)

	server := &http.Server{Addr: addr, Handler: mux}

	secured := ssl.Available()

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
		log:      log.WithField("context", "http"),
	}, nil
}

// Start server
func (s *HTTPServer) Start() error {
	if s.secured {
		s.log.Infof("Starting HTTPS server at %v", s.addr)
		return s.server.ListenAndServeTLS("", "")
	}

	s.log.Infof("Starting HTTP server at %v", s.addr)
	return s.server.ListenAndServe()
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
