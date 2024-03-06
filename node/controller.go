package node

import (
	"errors"
	"log/slog"

	"github.com/anycable/anycable-go/common"
)

// Controller is an interface describing business-logic handler (e.g. RPC)
type Controller interface {
	Start() error
	Shutdown() error
	Authenticate(sid string, env *common.SessionEnv) (*common.ConnectResult, error)
	Subscribe(sid string, env *common.SessionEnv, ids string, channel string) (*common.CommandResult, error)
	Unsubscribe(sid string, env *common.SessionEnv, ids string, channel string) (*common.CommandResult, error)
	Perform(sid string, env *common.SessionEnv, ids string, channel string, data string) (*common.CommandResult, error)
	Disconnect(sid string, env *common.SessionEnv, ids string, subscriptions []string) error
}

type NullController struct {
	log *slog.Logger
}

func NewNullController(l *slog.Logger) *NullController {
	return &NullController{l.With("context", "rpc", "impl", "null")}
}

func (c *NullController) Start() (err error) {
	c.log.Info("no RPC configured")

	return
}

func (c *NullController) Shutdown() (err error) { return }

func (c *NullController) Authenticate(sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	c.log.Debug("reject connection")

	return &common.ConnectResult{
		Status:             common.FAILURE,
		Transmissions:      []string{common.DisconnectionMessage(common.UNAUTHORIZED_REASON, false)},
		DisconnectInterest: -1,
	}, nil
}

func (c *NullController) Subscribe(sid string, env *common.SessionEnv, ids string, channel string) (*common.CommandResult, error) {
	c.log.Debug("reject subscription", "channel", channel)

	return &common.CommandResult{
		Status:             common.FAILURE,
		Transmissions:      []string{common.RejectionMessage(channel)},
		DisconnectInterest: -1,
	}, nil
}

func (c *NullController) Perform(sid string, env *common.SessionEnv, ids string, channel string, data string) (*common.CommandResult, error) {
	return nil, errors.New("not implemented")
}

func (c *NullController) Unsubscribe(sid string, env *common.SessionEnv, ids string, channel string) (*common.CommandResult, error) {
	return nil, errors.New("not implemented")
}

func (c *NullController) Disconnect(sid string, env *common.SessionEnv, ids string, subscriptions []string) error {
	return nil
}
