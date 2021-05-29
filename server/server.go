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
	log      *log.Entry

	Mux *http.ServeMux
}

var (
	allServers map[string]*HTTPServer = make(map[string]*HTTPServer)
	// Host is a default bind address for HTTP servers
	Host string = "localhost"
	// SSL is a default configuration for HTTP servers
	SSL *SSLConfig
	// MaxConn is a default configuration for maximum connections
	MaxConn int
)

// ForPort creates new or returns the existing server for the specified port
func ForPort(port string) (*HTTPServer, error) {
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

		server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cer}, MinVersion: tls.VersionTLS12}
	}

	return &HTTPServer{
		server:   server,
		addr:     addr,
		Mux:      mux,
		secured:  secured,
		shutdown: false,
		started:  false,
		maxConn:  maxConn,
		log:      log.WithField("context", "http"),
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
		s.log.Debugf("%s is mounted at %s", name, s.Address())
		return nil
	}

	s.log.Debugf("Starting %s at %s", name, s.Address())
	s.mu.Unlock()

	return s.Start()
}

// Running returns true if server has been started
func (s *HTTPServer) Running() bool {
	return s.started
}

// Shutdown shuts down server gracefully.
func (s *HTTPServer) Shutdown() error {
	s.mu.Lock()
	if s.shutdown {
		s.mu.Unlock()
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
