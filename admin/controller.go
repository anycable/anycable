package admin

import (
	"context"
	"log/slog"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/identity"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/version"
)

type AdminController struct {
	app        *App
	identifier *identity.JWTIdentifier
	conf       *Config
	log        *slog.Logger
}

var _ node.Controller = (*AdminController)(nil)

func NewAdminController(app *App, conf *Config, l *slog.Logger) *AdminController {
	var identifier *identity.JWTIdentifier

	if conf.Secret != "" {
		jconf := identity.NewJWTConfig(conf.Secret)
		jconf.Force = true
		identifier = identity.NewJWTIdentifier(&jconf, l)
	}

	return &AdminController{
		app:        app,
		identifier: identifier,
		conf:       conf,
		log:        l,
	}
}

func (c *AdminController) Start() error {
	return nil
}

func (c *AdminController) Shutdown() error {
	return nil
}

func (c *AdminController) Authenticate(ctx context.Context, sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	if c.identifier != nil {
		return c.identifier.Identify(sid, env)
	}

	return &common.ConnectResult{
		Identifier:    `{"sid":"` + sid + `","version":"` + version.Version() + `"}`,
		Transmissions: []string{common.WelcomeMessage(sid)},
		Status:        common.SUCCESS,
	}, nil
}

func (c *AdminController) Subscribe(ctx context.Context, sid string, env *common.SessionEnv, identifiers string, stream string) (*common.CommandResult, error) {
	if err := c.app.HandleSubscribe(sid, stream); err != nil {
		return nil, err
	}

	return &common.CommandResult{
		Status:             common.SUCCESS,
		Transmissions:      []string{common.ConfirmationMessage(stream)},
		Streams:            []string{stream},
		DisconnectInterest: -1,
	}, nil
}

func (c *AdminController) Unsubscribe(ctx context.Context, sid string, env *common.SessionEnv, identifiers string, channel string) (*common.CommandResult, error) {
	return &common.CommandResult{
		Status:         common.SUCCESS,
		Transmissions:  []string{},
		Streams:        []string{},
		StopAllStreams: true,
	}, nil
}

func (c *AdminController) Perform(ctx context.Context, sid string, env *common.SessionEnv, identifiers string, channel string, data string) (*common.CommandResult, error) {
	return &common.CommandResult{
		Status: common.FAILURE,
	}, nil
}

func (c *AdminController) Disconnect(ctx context.Context, sid string, env *common.SessionEnv, identifiers string, subscriptions []string) error {
	return c.app.HandleDisconnect(sid, subscriptions)
}
