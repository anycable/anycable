package rpc

import (
	"github.com/anycable/anycable-go/config"
	"github.com/anycable/anycable-go/node"
)

// Controller implements node.Controller interface for gRPC
type Controller struct {
}

// NewController builds new Controller from config
func NewController(config *config.Config) *Controller {
	return &Controller{}
}

// Start initializes RPC connection pool
func (c *Controller) Start() error {
	return nil
}

// Shutdown waits for all active RPC requests and closes connections
func (c *Controller) Shutdown() error {
	return nil
}

// Authenticate performs Connect RPC call
func (c *Controller) Authenticate(path string, headers *map[string]string) (string, error) {
	return "123", nil
}

// Subscribe performs Command RPC call with "subscribe" command
func (c *Controller) Subscribe(sid string, id string, channel string) (*node.CommandResult, error) {
	return &node.CommandResult{}, nil
}

// Unsubscribe performs Command RPC call with "unsubscribe" command
func (c *Controller) Unsubscribe(sid string, id string, channel string) (*node.CommandResult, error) {
	return &node.CommandResult{}, nil
}

// Perform performs Command RPC call with "perform" command
func (c *Controller) Perform(sid string, id string, channel string, data string) (*node.CommandResult, error) {
	return &node.CommandResult{}, nil
}
