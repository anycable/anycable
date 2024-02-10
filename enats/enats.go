//go:build !freebsd || amd64
// +build !freebsd amd64

package enats

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joomcode/errorx"
	gonanoid "github.com/matoous/go-nanoid"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// NewConfig returns defaults for NATSServiceConfig
func NewConfig() Config {
	return Config{
		ServiceAddr:           nats.DefaultURL,
		ClusterName:           "anycable-cluster",
		JetStreamReadyTimeout: 30, // seconds
	}
}

const (
	serverStartTimeout = 5 * time.Second
)

// Service represents NATS service
type Service struct {
	config *Config
	server *server.Server
	name   string

	mu sync.Mutex
}

// LogEntry represents LoggerV2 decorator for nats server logger
type LogEntry struct {
	*slog.Logger
}

// Debugf is an alias for Debug
func (e *LogEntry) Debugf(format string, v ...interface{}) {
	e.Debug(fmt.Sprintf(format, v...))
}

// Warnf is an alias for Warn
func (e *LogEntry) Warnf(format string, v ...interface{}) {
	e.Warn(fmt.Sprintf(format, v...))
}

// Errorf is an alias for Error
func (e *LogEntry) Errorf(format string, v ...interface{}) {
	e.Error(fmt.Sprintf(format, v...))
}

// Infof is an alias for Info
func (e *LogEntry) Infof(format string, v ...interface{}) {
	e.Info(fmt.Sprintf(format, v...))
}

// Fatalf is an alias for Fatal
func (e *LogEntry) Fatalf(format string, v ...interface{}) {
	log.Fatalf(format, v...)
}

// Noticef is an alias for Infof
func (e *LogEntry) Noticef(format string, v ...interface{}) {
	e.Info(fmt.Sprintf(format, v...))
}

// Tracef is an alias for Debugf
func (e *LogEntry) Tracef(format string, v ...interface{}) {
	e.Debug(fmt.Sprintf(format, v...))
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
		return errorx.Decorate(err, "error parsing NATS service addr")
	}
	if u.Port() == "" {
		return errorx.IllegalArgument.New("failed to parse NATS server URL, can not fetch port")
	}

	port, err := strconv.ParseInt(u.Port(), 10, 32)
	if err != nil {
		return errorx.Decorate(err, "failed to parse NATS service port")
	}

	clusterOpts, err := s.getCluster(s.config.ClusterAddr, s.config.ClusterName)
	if err != nil {
		return errorx.Decorate(err, "failed to configure NATS cluster")
	}

	routes, err := s.getRoutes()
	if err != nil {
		return errorx.Decorate(err, "failed to parse routes")
	}

	gatewayOpts, err := s.getGateway(s.config.GatewayAddr, s.config.GatewayAdvertise, s.config.ClusterName, s.config.Gateways)
	if err != nil {
		return errorx.Decorate(err, "failed to configure NATS gateway")
	}

	opts := &server.Options{
		Host:       u.Hostname(),
		Port:       int(port),
		Debug:      s.config.Debug,
		Trace:      s.config.Trace,
		ServerName: s.serverName(),
		Cluster:    clusterOpts,
		Gateway:    gatewayOpts,
		Routes:     routes,
		NoSigs:     true,
		JetStream:  s.config.JetStream,
	}

	if s.config.StoreDir != "" {
		opts.StoreDir = s.config.StoreDir
	} else {
		opts.StoreDir = filepath.Join(os.TempDir(), "nats-data", s.serverName())
	}

	s.server, err = server.NewServer(opts)
	if err != nil {
		return errorx.Decorate(err, "failed to start NATS server")
	}

	if s.config.Debug {
		e := &LogEntry{slog.With("service", "nats")}
		s.server.SetLogger(e, s.config.Debug, s.config.Trace)
	}

	go s.server.Start()

	return s.WaitReady()
}

// WaitReady waits while NATS server is starting
func (s *Service) WaitReady() error {
	if s.server.ReadyForConnections(serverStartTimeout) {
		// We don't want to block the bootstrap process while waiting for JetStream.
		// JetStream requires a cluster to be formed before it can be enabled, but when we
		// perform a rolling update, the newly created instance may have no network connectivity,
		// thus, it won't be able to join the cluster and enable JetStream.
		go s.WaitJetStreamReady(s.config.JetStreamReadyTimeout) // nolint:errcheck
		return nil
	}

	return errorx.TimeoutElapsed.New(
		"failed to start NATS server within %d seconds", serverStartTimeout,
	)
}

func (s *Service) Description() string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("server_name: %s", s.serverName()))

	if s.config.ClusterAddr != "" {
		builder.WriteString(fmt.Sprintf(", cluster: %s, cluster_name: %s", s.config.ClusterAddr, s.config.ClusterName))
	}

	if s.config.Routes != nil {
		builder.WriteString(fmt.Sprintf(", routes: %s", strings.Join(s.config.Routes, ",")))
	}

	if s.config.GatewayAddr != "" {
		builder.WriteString(fmt.Sprintf(", gateway: %s, gateways: %s", s.config.GatewayAddr, s.config.Gateways))

		if s.config.GatewayAdvertise != "" {
			builder.WriteString(fmt.Sprintf(", gateway_advertise: %s", s.config.GatewayAdvertise))
		}
	}

	return builder.String()
}

// Shutdown shuts the NATS server down
func (s *Service) Shutdown(ctx context.Context) error {
	s.server.DisableJetStream() // nolint:errcheck
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
			return nil, errorx.Decorate(err, "error parsing route URL")
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
		err = errorx.Decorate(err, "failed to parse cluster URL")
		return
	}

	opts = server.ClusterOpts{
		Name: name,
		Host: host,
		Port: port,
	}

	return
}

func (s *Service) getGateway(addr string, advertise string, name string, gateways []string) (opts server.GatewayOpts, err error) {
	if addr == "" || name == "" {
		return
	}

	host, port, err := parseAddress(addr)

	if err != nil {
		err = errorx.Decorate(err, "failed to parse gateway URL")
		return
	}

	opts = server.GatewayOpts{
		Name:      s.config.ClusterName,
		Host:      host,
		Port:      port,
		Advertise: advertise,
	}

	if len(gateways) != 0 {
		gateOpts := make([]*server.RemoteGatewayOpts, len(gateways))
		for i, g := range gateways {
			parts := strings.SplitN(g, ":", 2)

			if len(parts) != 2 {
				err = errorx.Decorate(err, "gateway has unknown format: %s", g)
				return
			}

			name := parts[0]
			addrs := strings.Split(parts[1], ",")

			nameAddrs := make([]*url.URL, len(addrs))

			for j, addr := range addrs {
				u, gateErr := url.Parse(addr)
				if gateErr != nil {
					err = errorx.Decorate(gateErr, "error parsing gateway URL")
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
		return "", 0, errorx.Decorate(err, "failed to parse URL")
	}

	if uri.Port() == "" {
		return "", 0, errorx.IllegalArgument.New("port cannot be empty")
	}

	port, err := strconv.ParseInt(uri.Port(), 10, 32)
	if err != nil {
		return "", 0, errorx.Decorate(err, "port is not valid")
	}

	return uri.Hostname(), int(port), nil
}

func (s *Service) serverName() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.name != "" {
		return s.name
	}

	if s.config.Name != "" {
		s.name = s.config.Name
		return s.name
	}

	suf, _ := gonanoid.Nanoid(3) // nolint: errcheck

	s.name = strings.ReplaceAll(strings.ReplaceAll(s.config.ServiceAddr, ":", "-"), "/", "") + "-" + suf
	return s.name
}

func (s *Service) WaitJetStreamReady(maxSeconds int) error {
	if !s.config.JetStream {
		return nil
	}

	start := time.Now()
	for {
		if time.Since(start) > time.Duration(maxSeconds)*time.Second {
			return fmt.Errorf("JetStream is not ready after %d seconds", maxSeconds)
		}

		c, err := nats.Connect("", nats.InProcessServer(s.server))
		if err != nil {
			slog.With("context", "enats").Debug("NATS server not accepting connections", "error", err)
			continue
		}

		j, err := c.JetStream()
		if err != nil {
			return err
		}

		st, err := j.StreamInfo("__anycable__ready__", nats.MaxWait(1*time.Second))
		if err == nats.ErrStreamNotFound || st != nil {
			leader := s.server.JetStreamIsLeader()
			slog.With("context", "enats").Debug("JetStream cluster is ready", "leader", leader)
			return nil
		}

		c.Close()

		slog.With("context", "enats").Debug("JetStream cluster is not ready yet, waiting for 1 second...")

		time.Sleep(1 * time.Second)
	}
}
