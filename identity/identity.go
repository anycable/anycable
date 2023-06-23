package identity

import (
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

func (c *IdentifiableController) Authenticate(sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	res, err := c.identifier.Identify(sid, env)

	if err != nil {
		return nil, err
	}

	// Passthrough
	if res == nil {
		return c.controller.Authenticate(sid, env)
	}

	if res.CState == nil {
		res.CState = make(map[string]string)
	}

	res.DisconnectInterest = -1

	return res, err
}

func (c *IdentifiableController) Subscribe(sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
	return c.controller.Subscribe(sid, env, id, channel)
}

func (c *IdentifiableController) Unsubscribe(sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
	return c.controller.Unsubscribe(sid, env, id, channel)
}

func (c *IdentifiableController) Perform(sid string, env *common.SessionEnv, id string, channel string, data string) (*common.CommandResult, error) {
	return c.controller.Perform(sid, env, id, channel, data)
}
func (c *IdentifiableController) Disconnect(sid string, env *common.SessionEnv, id string, subscriptions []string) error {
	return c.controller.Disconnect(sid, env, id, subscriptions)
}

func actionCableWelcomeMessage(sid string) string {
	return fmt.Sprintf(actionCableWelcomeMessageTemplate, sid)
}
