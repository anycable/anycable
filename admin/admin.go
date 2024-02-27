package admin

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/logger"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/pubsub"
	"github.com/anycable/anycable-go/server"
	"github.com/golang-jwt/jwt"
)

type App struct {
	appnode *node.Node
	server  *server.HTTPServer

	node *node.Node

	conf   *Config
	log    *slog.Logger
	tracer *logger.Tracer

	mu sync.Mutex
}

type AdminOption func(*App)

func WithLogger(l *slog.Logger) AdminOption {
	return func(a *App) {
		a.log = l.With("context", "admin")
	}
}

func WithTracer(t *logger.Tracer) AdminOption {
	return func(a *App) {
		a.tracer = t
	}
}

func NewApp(n *node.Node, m *metrics.Metrics, c *Config, opt ...AdminOption) (*App, error) {
	var s *server.HTTPServer

	if c.Port != 0 {
		srv, err := server.ForPort(strconv.Itoa(c.Port))

		if err != nil {
			return nil, err
		}

		s = srv
	}

	app := &App{
		appnode: n,
		server:  s,
		conf:    c,
	}

	for _, o := range opt {
		o(app)
	}

	if app.log == nil {
		app.log = slog.Default().With("context", "admin")
	}

	// Set up standalone admin node
	nconf := node.NewConfig()
	// Do not refresh stats from this node
	nconf.StatsRefreshInterval = 0
	controller := NewAdminController(app, c, app.log)
	app.node = node.NewNode(
		&nconf,
		node.WithLogger(app.log),
		node.WithController(controller),
		node.WithInstrumenter(m),
	)

	disconnector := node.NewInlineDisconnector(app.node)
	app.node.SetDisconnector(disconnector)

	ps := pubsub.NewLegacySubscriber(app.node)
	broker := broker.NewLegacyBroker(ps)
	app.node.SetBroker(broker)

	return app, nil
}

func (app *App) Run() error {
	if app.tracer != nil {
		go app.tracer.Run(app.broadcastLogs)
	}

	app.node.Start() // nolint:errcheck

	if app.server != nil {
		err := app.MountHandlers(app.server)

		if err != nil {
			return err
		}

		app.log.Info(fmt.Sprintf("Run admin console at %s%s", app.server.Address(), app.conf.Path))

		if err := app.server.StartAndAnnounce("Admin server"); err != nil {
			if !app.server.Stopped() {
				return fmt.Errorf("admin HTTP server at %s stopped: %v", app.server.Address(), err)
			}
		}
	}

	return nil
}

func (app *App) Shutdown(ctx context.Context) error {
	app.mu.Lock()
	defer app.mu.Unlock()

	if app.server != nil {
		app.log.Info("Shutting down HTTP server")
		err := app.server.Shutdown(ctx)
		if err != nil {
			app.log.Error("Error shutting down HTTP server", "err", err)
		}
	}

	return app.node.Shutdown(ctx)
}

func (app *App) NewEventsURL(stream string) string {
	base := url.URL{
		Path: path.Join(app.conf.Path, ssePath),
	}

	query := url.Values{}
	if app.conf.Secret != "" {
		token, err := app.newEventsToken()
		if err == nil {
			query.Add("jid", token)
		} else {
			app.log.Error("failed to create JWT token", "error", err)
		}
	}

	query.Add("identifier", stream)

	base.RawQuery = query.Encode()

	return base.String()
}

func (app *App) newEventsToken() (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"ext": "{}",
		"exp": time.Now().Local().Add(time.Minute * time.Duration(15)).Unix(),
	})

	return token.SignedString([]byte(app.conf.Secret))
}
