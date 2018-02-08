package server

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/anycable/anycable-go/config"
)

// HTTPServer is wrapper over http.Server
type HTTPServer struct {
	server  *http.Server
	secured bool
	mux     *http.ServeMux
}

// NewServer builds HTTPServer from config params
func NewServer(host string, port string, ssl *config.SSLOptions) (*HTTPServer, error) {
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

	return &HTTPServer{server: server, mux: mux, secured: secured}, nil
}

// Start server
func (s *HTTPServer) Start() error {
	if s.secured {
		return s.server.ListenAndServeTLS("", "")
	}

	return s.server.ListenAndServe()
}
