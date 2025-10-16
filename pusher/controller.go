package pusher

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/utils"
	"github.com/joomcode/errorx"
)

type SubscribeRequest struct {
	Channel string `json:"channel"`
	Stream  string `json:"stream"`
}

type PresenceInfo struct {
	Count int                    `json:"count"`
	IDs   []string               `json:"ids"`
	Hash  map[string]interface{} `json:"hash"`
}

type UserPresence struct {
	UserID   string      `json:"user_id"`
	UserInfo interface{} `json:"user_info"`
}

type Controller struct {
	conf   *Config
	broker broker.Broker
	log    *slog.Logger
}

var _ node.Controller = (*Controller)(nil)

// This controller is responsible for terminating Pusher commands and properly handling private-, presence- and public channels.
// IMPORTANT: Authorization happens in the Executor (because we have to support different identifiers for subscribe and other commands)
func NewController(b broker.Broker, c *Config, l *slog.Logger) *Controller {
	return &Controller{c, b, l.With("context", "pusher")}
}

func (c *Controller) Start() error {
	return nil
}

func (c *Controller) Shutdown() error {
	return nil
}

func (c *Controller) Authenticate(ctx context.Context, sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	// TODO: shouldn't it be pusher:signin handler?
	return nil, nil
}

func (c *Controller) Subscribe(ctx context.Context, sid string, env *common.SessionEnv, ids string, identifier string) (*common.CommandResult, error) {
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
	var presenceEvent *common.PresenceEvent

	confirmationMsg := &common.Reply{
		Type:       common.ConfirmedType,
		Identifier: identifier,
	}

	if strings.HasPrefix(request.Stream, "private-") || strings.HasPrefix(request.Stream, "presence-") {
		state = make(map[string]string)
		state[common.WHISPER_STREAM_STATE] = request.Stream
	}

	if strings.HasPrefix(request.Stream, "presence-") {
		if state == nil {
			state = make(map[string]string)
		}
		state[common.PRESENCE_STREAM_STATE] = request.Stream

		// Extract join event from the state
		if presenceData := env.GetChannelStateField(identifier, "channel_data"); presenceData != "" {
			var userPresence *UserPresence

			err := json.Unmarshal([]byte(presenceData), &userPresence)
			if err != nil {
				return &common.CommandResult{
					Status: common.ERROR,
				}, errorx.Decorate(err, "failed to unmarshal presence data")
			}

			presenceEvent = &common.PresenceEvent{Type: common.PresenceJoinType, ID: userPresence.UserID, Info: userPresence.UserInfo}
		}

		if c.broker != nil {
			info, err := c.broker.PresenceInfo(request.Stream)
			if err != nil {
				return &common.CommandResult{
					Status: common.ERROR,
				}, errorx.Decorate(err, "failed to load presence info")
			}

			pusherInfo := PresenceInfo{Count: info.Total, IDs: make([]string, info.Total), Hash: make(map[string]interface{}, info.Total)}

			for i, r := range info.Records {
				pusherInfo.IDs[i] = r.ID
				pusherInfo.Hash[r.ID] = r.Info
			}

			// Add the current presence if missing
			if presenceEvent != nil {
				if _, ok := pusherInfo.Hash[presenceEvent.ID]; !ok {
					pusherInfo.IDs = append(pusherInfo.IDs, presenceEvent.ID)
					pusherInfo.Hash[presenceEvent.ID] = presenceEvent.Info
					pusherInfo.Count++
				}
			}

			confirmationMsg.Message = string(utils.ToJSON(map[string]interface{}{"presence": pusherInfo}))
		}
	}

	confirmation := string(utils.ToJSON(confirmationMsg))

	return &common.CommandResult{
		Status:             common.SUCCESS,
		Transmissions:      []string{confirmation},
		Streams:            []string{request.Stream},
		DisconnectInterest: -1,
		IState:             state,
		Presence:           presenceEvent,
	}, nil
}

func (c *Controller) Unsubscribe(ctx context.Context, sid string, env *common.SessionEnv, ids string, identifier string) (*common.CommandResult, error) {
	return &common.CommandResult{
		Status:         common.SUCCESS,
		Transmissions:  []string{},
		Streams:        []string{},
		StopAllStreams: true,
	}, nil
}

func (c *Controller) Perform(ctx context.Context, sid string, env *common.SessionEnv, ids string, identifier string, data string) (*common.CommandResult, error) {
	return nil, nil
}

func (c *Controller) Disconnect(ctx context.Context, sid string, env *common.SessionEnv, ids string, subscriptions []string) error {
	return nil
}
