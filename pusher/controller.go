package pusher

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/joomcode/errorx"
)

type SubscribeRequest struct {
	Channel string `json:"channel"`
	Stream  string `json:"stream"`
}

type Controller struct {
	conf *Config
	log  *slog.Logger
}

var _ node.Controller = (*Controller)(nil)

// This controller is responsible for terminating Pusher commands and properly handling private-, presence- and public channels.
// IMPORTANT: Authorization happens in the Executor (because we have to support different identifiers for subscribe and other commands)
func NewController(conf *Config, l *slog.Logger) *Controller {
	return &Controller{conf, l.With("context", "pusher")}
}

func (c *Controller) Start() error {
	return nil
}

func (c *Controller) Shutdown() error {
	return nil
}

func (c *Controller) Authenticate(sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	// TODO: shouldn't it be pusher:signin handler?
	return nil, nil
}

func (c *Controller) Subscribe(sid string, env *common.SessionEnv, ids string, identifier string) (*common.CommandResult, error) {
	var request SubscribeRequest

	err := json.Unmarshal([]byte(identifier), &request)

	if err != nil {
		return &common.CommandResult{
			Status:        common.FAILURE,
			Transmissions: []string{common.RejectionMessage(identifier)},
		}, errorx.Decorate(err, "invalid identifier")
	}

	if request.Stream == "" {
		return &common.CommandResult{
			Status:        common.FAILURE,
			Transmissions: []string{common.RejectionMessage(identifier)},
		}, nil
	}

	var state map[string]string

	if strings.HasPrefix(request.Stream, "private-") || strings.HasPrefix(request.Stream, "presence-") {
		state = make(map[string]string)
		state[common.WHISPER_STREAM_STATE] = request.Stream
	}

	if strings.HasPrefix(request.Stream, "presence-") {
		if state == nil {
			state = make(map[string]string)
		}
		state[common.PRESENCE_STREAM_STATE] = request.Stream
	}

	return &common.CommandResult{
		Status:             common.SUCCESS,
		Transmissions:      []string{common.ConfirmationMessage(identifier)},
		Streams:            []string{request.Stream},
		DisconnectInterest: -1,
		IState:             state,
	}, nil
}

func (c *Controller) Unsubscribe(sid string, env *common.SessionEnv, ids string, identifier string) (*common.CommandResult, error) {
	return &common.CommandResult{
		Status:         common.SUCCESS,
		Transmissions:  []string{},
		Streams:        []string{},
		StopAllStreams: true,
	}, nil
}

func (c *Controller) Perform(sid string, env *common.SessionEnv, ids string, identifier string, data string) (*common.CommandResult, error) {
	return nil, nil
}

func (c *Controller) Disconnect(sid string, env *common.SessionEnv, ids string, subscriptions []string) error {
	return nil
}
