package router

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
)

type RouterController struct {
	controller node.Controller
	routes     map[string]node.Controller
}

var _ node.Controller = (*RouterController)(nil)

func NewRouterController(c node.Controller) *RouterController {
	return &RouterController{c, make(map[string]node.Controller)}
}

func (c *RouterController) SetDefault(controller node.Controller) {
	c.controller = controller
}

func (c *RouterController) Route(channel string, handler node.Controller) error {
	if _, ok := c.routes[channel]; ok {
		return fmt.Errorf("Route has been already defined: %s", channel)
	}

	c.routes[channel] = handler

	return nil
}

func (c *RouterController) Empty() bool {
	return len(c.routes) == 0
}

func (c *RouterController) Routes() []string {
	keys := []string{}
	for k := range c.routes {
		keys = append(keys, k)
	}

	return keys
}

func (c *RouterController) Start() error {
	return c.controller.Start()
}

func (c *RouterController) Shutdown() error {
	return c.controller.Shutdown()
}

func (c *RouterController) Authenticate(ctx context.Context, sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	return c.controller.Authenticate(ctx, sid, env)
}

func (c *RouterController) Subscribe(ctx context.Context, sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
	channelName := extractChannel(channel)

	if channelName != "" {
		if handler, ok := c.routes[channelName]; ok {
			res, err := handler.Subscribe(ctx, sid, env, id, channel)

			if res != nil || err != nil {
				return res, err
			}
		}
	}

	return c.controller.Subscribe(ctx, sid, env, id, channel)
}

func (c *RouterController) Unsubscribe(ctx context.Context, sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
	channelName := extractChannel(channel)

	if channelName != "" {
		if handler, ok := c.routes[channelName]; ok {
			res, err := handler.Unsubscribe(ctx, sid, env, id, channel)

			if res != nil || err != nil {
				return res, err
			}
		}
	}

	return c.controller.Unsubscribe(ctx, sid, env, id, channel)
}

func (c *RouterController) Perform(ctx context.Context, sid string, env *common.SessionEnv, id string, channel string, data string) (*common.CommandResult, error) {
	channelName := extractChannel(channel)

	if channelName != "" {
		if handler, ok := c.routes[channelName]; ok {
			res, err := handler.Perform(ctx, sid, env, id, channel, data)

			if res != nil || err != nil {
				return res, err
			}
		}
	}

	return c.controller.Perform(ctx, sid, env, id, channel, data)
}
func (c *RouterController) Disconnect(ctx context.Context, sid string, env *common.SessionEnv, id string, subscriptions []string) error {
	return c.controller.Disconnect(ctx, sid, env, id, subscriptions)
}

func extractChannel(identifier string) string {
	params := struct {
		Channel string `json:"channel"`
	}{}

	err := json.Unmarshal([]byte(identifier), &params)

	if err != nil {
		return ""
	}

	return params.Channel
}
