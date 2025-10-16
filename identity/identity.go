package identity

import (
	"context"
	"fmt"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
)

const (
	actionCableWelcomeMessageTemplate        = "{\"type\":\"welcome\",\"sid\":\"%s\"}"
	actionCableDisconnectUnauthorizedMessage = "{\"type\":\"disconnect\",\"reason\":\"unauthorized\",\"reconnect\":false}"
)

//go:generate mockery --name Identifier --output "../mocks" --outpkg mocks
type Identifier interface {
	Identify(sid string, env *common.SessionEnv) (*common.ConnectResult, error)
}

type IdentifierPipeline struct {
	identifiers []Identifier
}

var _ Identifier = (*IdentifierPipeline)(nil)

func NewIdentifierPipeline(identifiers ...Identifier) *IdentifierPipeline {
	return &IdentifierPipeline{identifiers}
}

func (p *IdentifierPipeline) Identify(sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	for _, i := range p.identifiers {
		res, err := i.Identify(sid, env)

		if err != nil || res != nil {
			return res, err
		}
	}

	return nil, nil
}

type IdentifiableController struct {
	controller node.Controller
	identifier Identifier
}

var _ node.Controller = (*IdentifiableController)(nil)

func NewIdentifiableController(c node.Controller, i Identifier) *IdentifiableController {
	return &IdentifiableController{c, i}
}

func (c *IdentifiableController) Start() error {
	return c.controller.Start()
}

func (c *IdentifiableController) Shutdown() error {
	return c.controller.Shutdown()
}

func (c *IdentifiableController) Authenticate(ctx context.Context, sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	res, err := c.identifier.Identify(sid, env)

	if err != nil {
		return nil, err
	}

	// Passthrough
	if res == nil {
		return c.controller.Authenticate(ctx, sid, env)
	}

	if res.CState == nil {
		res.CState = make(map[string]string)
	}

	res.DisconnectInterest = -1

	return res, err
}

func (c *IdentifiableController) Subscribe(ctx context.Context, sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
	return c.controller.Subscribe(ctx, sid, env, id, channel)
}

func (c *IdentifiableController) Unsubscribe(ctx context.Context, sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
	return c.controller.Unsubscribe(ctx, sid, env, id, channel)
}

func (c *IdentifiableController) Perform(ctx context.Context, sid string, env *common.SessionEnv, id string, channel string, data string) (*common.CommandResult, error) {
	return c.controller.Perform(ctx, sid, env, id, channel, data)
}
func (c *IdentifiableController) Disconnect(ctx context.Context, sid string, env *common.SessionEnv, id string, subscriptions []string) error {
	return c.controller.Disconnect(ctx, sid, env, id, subscriptions)
}

func actionCableWelcomeMessage(sid string) string {
	return fmt.Sprintf(actionCableWelcomeMessageTemplate, sid)
}
