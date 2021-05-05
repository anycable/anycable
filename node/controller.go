package node

import "github.com/anycable/anycable-go/common"

// Controller is an interface describing business-logic handler (e.g. RPC)
type Controller interface {
	Start() error
	Shutdown() error
	Authenticate(sid string, env *common.SessionEnv) (*common.ConnectResult, error)
	Subscribe(sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error)
	Unsubscribe(sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error)
	Perform(sid string, env *common.SessionEnv, id string, channel string, data string) (*common.CommandResult, error)
	Disconnect(sid string, env *common.SessionEnv, id string, subscriptions []string) error
}
