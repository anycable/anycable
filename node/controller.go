package node

import (
	"context"
	"errors"
	"log/slog"

	"github.com/anycable/anycable-go/common"
)

// Controller is an interface describing business-logic handler (e.g. RPC)
//
//go:generate mockery --name Controller --output "../mocks" --outpkg mocks
type Controller interface {
	Start() error
	Shutdown() error
	Authenticate(ctx context.Context, sid string, env *common.SessionEnv) (*common.ConnectResult, error)
	Subscribe(ctx context.Context, sid string, env *common.SessionEnv, ids string, channel string) (*common.CommandResult, error)
	Unsubscribe(ctx context.Context, sid string, env *common.SessionEnv, ids string, channel string) (*common.CommandResult, error)
	Perform(ctx context.Context, sid string, env *common.SessionEnv, ids string, channel string, data string) (*common.CommandResult, error)
	Disconnect(ctx context.Context, sid string, env *common.SessionEnv, ids string, subscriptions []string) error
}

type NullController struct {
	log *slog.Logger
}

var _ Controller = (*NullController)(nil)

func NewNullController(l *slog.Logger) *NullController {
	return &NullController{l.With("context", "rpc", "impl", "null")}
}

func (c *NullController) Start() (err error) {
	c.log.Info("no RPC configured")

	return
}

func (c *NullController) Shutdown() (err error) { return }

func (c *NullController) Authenticate(ctx context.Context, sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	c.log.Debug("reject connection")

	return &common.ConnectResult{
		Status:             common.FAILURE,
		Transmissions:      []string{common.DisconnectionMessage(common.UNAUTHORIZED_REASON, false)},
		DisconnectInterest: -1,
	}, nil
}

func (c *NullController) Subscribe(ctx context.Context, sid string, env *common.SessionEnv, ids string, channel string) (*common.CommandResult, error) {
	c.log.Debug("reject subscription", "channel", channel)

	return &common.CommandResult{
		Status:             common.FAILURE,
		Transmissions:      []string{common.RejectionMessage(channel)},
		DisconnectInterest: -1,
	}, nil
}

func (c *NullController) Perform(ctx context.Context, sid string, env *common.SessionEnv, ids string, channel string, data string) (*common.CommandResult, error) {
	return nil, errors.New("not implemented")
}

func (c *NullController) Unsubscribe(ctx context.Context, sid string, env *common.SessionEnv, ids string, channel string) (*common.CommandResult, error) {
	return nil, errors.New("not implemented")
}

func (c *NullController) Disconnect(ctx context.Context, sid string, env *common.SessionEnv, ids string, subscriptions []string) error {
	return nil
}
