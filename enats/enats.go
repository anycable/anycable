package enats

import (
	"net/url"
	"strconv"
	"time"

	"github.com/apex/log"
	"github.com/joomcode/errorx"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

const (
	serverStartTimeout = 5 * time.Second
)

// Config represents NATS service configuration
type Config struct {
	Debug       bool
	Trace       bool
	ServiceAddr string
	ClusterAddr string
	ClusterName string
	Routes      []string
}

// Service represents NATS service
type Service struct {
	config Config
	server *server.Server
}

// LogEntry represents LoggerV2 decorator for nats server logger
type LogEntry struct {
	*log.Entry
}

// Noticef is an alias for Infof
func (e *LogEntry) Noticef(format string, v ...interface{}) {
	e.Infof(format, v...)
}

// Tracef is an alias for Debugf
func (e *LogEntry) Tracef(format string, v ...interface{}) {
	e.Debugf(format, v...)
}

// NewConfig returns defaults for NATSServiceConfig
func NewConfig() Config {
	return Config{ServiceAddr: nats.DefaultURL, ClusterName: "anycable-cluster"}
}

// NewService returns an instance of NATS service
func NewService(c Config) *Service {
	return &Service{config: c}
}

// Start starts the service
func (s *Service) Start() error {
	var clusterOpts server.ClusterOpts
	var err error

	u, err := url.Parse(s.config.ServiceAddr)
	if err != nil {
		return errorx.Decorate(err, "Error parsing NATS service addr")
	}
	if u.Port() == "" {
		return errorx.IllegalArgument.New("Failed to parse NATS server URL, can not fetch port")
	}

	port, err := strconv.ParseInt(u.Port(), 10, 32)
	if err != nil {
		return errorx.Decorate(err, "Failed to parse NATS service port")
	}

	if s.config.ClusterAddr != "" {
		var clusterURL *url.URL
		var clusterPort int64

		clusterURL, err = url.Parse(s.config.ClusterAddr)
		if err != nil {
			return errorx.Decorate(err, "Failed to parse NATS cluster URL")
		}
		if clusterURL.Port() == "" {
			return errorx.IllegalArgument.New("Failed to parse NATS cluster port")
		}

		clusterPort, err = strconv.ParseInt(clusterURL.Port(), 10, 32)
		if err != nil {
			return errorx.Decorate(err, "Failed to parse NATS cluster port")
		}
		clusterOpts = server.ClusterOpts{
			Name: s.config.ClusterName,
			Host: clusterURL.Hostname(),
			Port: int(clusterPort),
		}
	}

	routes, err := s.getRoutes()
	if err != nil {
		return errorx.Decorate(err, "Failed to parse routes")
	}

	opts := &server.Options{
		Host:    u.Hostname(),
		Port:    int(port),
		Debug:   s.config.Debug,
		Trace:   s.config.Trace,
		Cluster: clusterOpts,
		Routes:  routes,
		NoSigs:  true,
	}

	s.server, err = server.NewServer(opts)
	if err != nil {
		return errorx.Decorate(err, "Failed to start NATS server")
	}

	if s.config.Debug {
		e := &LogEntry{log.WithField("service", "nats")}
		s.server.SetLogger(e, s.config.Debug, s.config.Trace)
	}

	go s.server.Start()

	return s.WaitReady()
}

// WaitReady waits while NATS server is starting
func (s *Service) WaitReady() error {
	if s.server.ReadyForConnections(serverStartTimeout) {
		return nil
	}

	return errorx.TimeoutElapsed.New(
		"Failed to start NATS server within %d seconds", serverStartTimeout,
	)
}

// Shutdown shuts the NATS server down
func (s *Service) Shutdown() error {
	s.server.Shutdown()
	s.server.WaitForShutdown()
	return nil
}

// getRoutes transforms []string routes to []*url.URL routes
func (s *Service) getRoutes() ([]*url.URL, error) {
	if len(s.config.Routes) == 0 {
		return nil, nil
	}

	routes := make([]*url.URL, len(s.config.Routes))
	for i, r := range s.config.Routes {
		u, err := url.Parse(r)
		if err != nil {
			return nil, errorx.Decorate(err, "Error parsing route URL")
		}
		routes[i] = u
	}
	return routes, nil
}
