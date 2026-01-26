package streams

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/utils"
	"github.com/joomcode/errorx"
)

type SubscribeRequest struct {
	StreamName       string `json:"stream_name"`
	SignedStreamName string `json:"signed_stream_name"`

	whisper  bool
	presence bool
}

func (r *SubscribeRequest) IsPresent() bool {
	return r.StreamName != "" || r.SignedStreamName != ""
}

type StreamResolver = func(string) (*SubscribeRequest, error)

type Controller struct {
	verifier *utils.MessageVerifier
	resolver StreamResolver
	log      *slog.Logger
}

var _ node.Controller = (*Controller)(nil)

func NewController(key string, resolver StreamResolver, l *slog.Logger) *Controller {
	verifier := utils.NewMessageVerifier(key)

	return &Controller{verifier, resolver, l.With("context", "streams")}
}

func (c *Controller) Start() error {
	return nil
}

func (c *Controller) Shutdown() error {
	return nil
}

func (c *Controller) Authenticate(ctx context.Context, sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	return nil, nil
}

func (c *Controller) Subscribe(ctx context.Context, sid string, env *common.SessionEnv, ids string, identifier string) (*common.CommandResult, error) {
	request, stream, err := c.VerifiedStream(identifier)

	if err != nil {
		c.log.With("identifier", identifier).Debug("subscription failed", "error", err)

		return &common.CommandResult{
				Status:        common.FAILURE,
				Transmissions: []string{common.RejectionMessage(identifier)},
			},
			err
	}

	if request.SignedStreamName != "" {
		c.log.With("identifier", identifier).Debug("subscribed verified", "stream", stream)
	} else {
		c.log.With("identifier", identifier).Debug("", "stream", stream)
	}

	var state map[string]string

	if request.whisper {
		state = make(map[string]string)
		state[common.WHISPER_STREAM_STATE] = stream
	}

	if request.presence {
		if state == nil {
			state = make(map[string]string)
		}
		state[common.PRESENCE_STREAM_STATE] = stream
	}

	return &common.CommandResult{
		Status:             common.SUCCESS,
		Transmissions:      []string{common.ConfirmationMessage(identifier)},
		Streams:            []string{stream},
		DisconnectInterest: -1,
		IState:             state,
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

func (c *Controller) VerifiedStream(identifier string) (*SubscribeRequest, string, error) {
	request, err := c.resolver(identifier)

	if err != nil {
		return nil, "", errorx.Decorate(err, "invalid identifier")
	}

	if !request.IsPresent() {
		return nil, "", errors.New("malformed identifier: no stream name or signed stream")
	}

	var stream string

	if request.SignedStreamName == "" {
		stream = request.StreamName

		c.log.With("identifier", identifier).Debug("unsigned", "stream", stream)
	} else {
		verified, err := c.verifier.Verified(request.SignedStreamName)

		if err != nil {
			return nil, "", errors.New("verification failed")
		}

		var ok bool
		stream, ok = verified.(string)

		if !ok {
			return nil, "", errors.New("verification failed: stream name is not a string")
		}
	}

	return request, stream, nil
}

func NewStreamsController(conf *Config, l *slog.Logger) *Controller {
	key := conf.Secret
	allowPublic := conf.Public
	whispers := conf.Whisper
	presence := conf.Presence

	resolver := func(identifier string) (*SubscribeRequest, error) {
		var request SubscribeRequest

		if err := json.Unmarshal([]byte(identifier), &request); err != nil {
			return nil, err
		}

		publicStream := request.StreamName != "" && request.SignedStreamName == ""

		if !allowPublic && publicStream {
			return nil, errors.New("public streams are not allowed")
		}

		if whispers || publicStream {
			request.whisper = true
		}

		if presence || publicStream {
			request.presence = true
		}

		return &request, nil
	}

	return NewController(key, resolver, l)
}

type TurboMessage struct {
	SignedStreamName string `json:"signed_stream_name"`
}

func NewTurboController(key string, l *slog.Logger) *Controller {
	resolver := func(identifier string) (*SubscribeRequest, error) {
		var msg TurboMessage

		if err := json.Unmarshal([]byte(identifier), &msg); err != nil {
			return nil, err
		}

		return &SubscribeRequest{SignedStreamName: msg.SignedStreamName}, nil
	}

	return NewController(key, resolver, l)
}

type CableReadyMesssage struct {
	Identifier string `json:"identifier"`
}

func NewCableReadyController(key string, l *slog.Logger) *Controller {
	resolver := func(identifier string) (*SubscribeRequest, error) {
		var msg CableReadyMesssage

		if err := json.Unmarshal([]byte(identifier), &msg); err != nil {
			return nil, err
		}

		return &SubscribeRequest{SignedStreamName: msg.Identifier}, nil
	}

	return NewController(key, resolver, l)
}
