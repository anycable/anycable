//go:build !freebsd || amd64
// +build !freebsd amd64

package enats

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/joomcode/errorx"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

const (
	serverStartTimeout = 5 * time.Second
)

// Service represents NATS service
type Service struct {
	config *Config
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
	return Config{
		ServiceAddr: nats.DefaultURL,
		ClusterName: "anycable-cluster",
	}
}

// NewService returns an instance of NATS service
func NewService(c *Config) *Service {
	return &Service{config: c}
}

// Start starts the service
func (s *Service) Start() error {
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

	clusterOpts, err := s.getCluster(s.config.ClusterAddr, s.config.ClusterName)
	if err != nil {
		return errorx.Decorate(err, "Failed to configure NATS cluster")
	}

	routes, err := s.getRoutes()
	if err != nil {
		return errorx.Decorate(err, "Failed to parse routes")
	}

	gatewayOpts, err := s.getGateway(s.config.GatewayAddr, s.config.ClusterName, s.config.Gateways)
	if err != nil {
		return errorx.Decorate(err, "Failed to configure NATS gateway")
	}

	opts := &server.Options{
		Host:    u.Hostname(),
		Port:    int(port),
		Debug:   s.config.Debug,
		Trace:   s.config.Trace,
		Cluster: clusterOpts,
		Gateway: gatewayOpts,
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

func (s *Service) Description() string {
	var builder strings.Builder

	if s.config.ClusterAddr != "" {
		builder.WriteString(fmt.Sprintf("cluster: %s, cluster_name: %s", s.config.ClusterAddr, s.config.ClusterName))
	}

	if s.config.Routes != nil {
		builder.WriteString(fmt.Sprintf(", routes: %s", strings.Join(s.config.Routes, ",")))
	}

	if s.config.GatewayAddr != "" {
		builder.WriteString(fmt.Sprintf(", gateway: %s, gateways: %s", s.config.GatewayAddr, s.config.Gateways))
	}

	return builder.String()
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

func (s *Service) getCluster(addr string, name string) (opts server.ClusterOpts, err error) {
	if addr == "" || name == "" {
		return
	}

	host, port, err := parseAddress(addr)

	if err != nil {
		err = errorx.Decorate(err, "Failed to parse cluster URL")
		return
	}

	opts = server.ClusterOpts{
		Name: name,
		Host: host,
		Port: port,
	}

	return
}

func (s *Service) getGateway(addr string, name string, gateways []string) (opts server.GatewayOpts, err error) {
	if addr == "" || name == "" {
		return
	}

	host, port, err := parseAddress(addr)

	if err != nil {
		err = errorx.Decorate(err, "Failed to parse gateway URL")
		return
	}

	opts = server.GatewayOpts{
		Name: s.config.ClusterName,
		Host: host,
		Port: port,
	}

	if len(gateways) != 0 {
		gateOpts := make([]*server.RemoteGatewayOpts, len(gateways))
		for i, g := range gateways {
			parts := strings.SplitN(g, ":", 2)

			if len(parts) != 2 {
				err = errorx.Decorate(err, "Gateway has unknown format: %s", g)
				return
			}

			name := parts[0]
			addrs := strings.Split(parts[1], ",")

			nameAddrs := make([]*url.URL, len(addrs))

			for j, addr := range addrs {
				u, gateErr := url.Parse(addr)
				if gateErr != nil {
					err = errorx.Decorate(gateErr, "Error parsing gateway URL")
					return
				}

				nameAddrs[j] = u
			}

			gateOpts[i] = &server.RemoteGatewayOpts{URLs: nameAddrs, Name: name}
		}

		opts.Gateways = gateOpts
	}

	return
}

func parseAddress(addr string) (string, int, error) {
	var uri *url.URL

	uri, err := url.Parse(addr)
	if err != nil {
		return "", 0, errorx.Decorate(err, "Failed to parse URL")
	}

	if uri.Port() == "" {
		return "", 0, errorx.IllegalArgument.New("Port cannot be empty")
	}

	port, err := strconv.ParseInt(uri.Port(), 10, 32)
	if err != nil {
		return "", 0, errorx.Decorate(err, "Port is not valid")
	}

	return uri.Hostname(), int(port), nil
}
