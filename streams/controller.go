package streams

import (
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
	var verifier *utils.MessageVerifier

	if key != "" {
		verifier = utils.NewMessageVerifier(key)
	}

	return &Controller{verifier, resolver, l.With("context", "streams")}
}

func (c *Controller) Start() error {
	return nil
}

func (c *Controller) Shutdown() error {
	return nil
}

func (c *Controller) Authenticate(sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	return nil, nil
}

func (c *Controller) Subscribe(sid string, env *common.SessionEnv, ids string, identifier string) (*common.CommandResult, error) {
	request, err := c.resolver(identifier)

	if err != nil {
		return &common.CommandResult{
			Status:        common.FAILURE,
			Transmissions: []string{common.RejectionMessage(identifier)},
		}, errorx.Decorate(err, "invalid identifier")
	}

	if !request.IsPresent() {
		err := errors.New("malformed identifier: no stream name or signed stream")

		return &common.CommandResult{
			Status:        common.FAILURE,
			Transmissions: []string{common.RejectionMessage(identifier)},
		}, err
	}

	var stream string

	if request.StreamName != "" {
		stream = request.StreamName

		c.log.With("identifier", identifier).Debug("unsigned", "stream", stream)
	} else {
		verified, err := c.verifier.Verified(request.SignedStreamName)

		if err != nil {
			c.log.With("identifier", identifier).Debug("verification failed", "stream", request.SignedStreamName, "error", err)

			return &common.CommandResult{
					Status:        common.FAILURE,
					Transmissions: []string{common.RejectionMessage(identifier)},
				},
				nil
		}

		var ok bool
		stream, ok = verified.(string)

		if !ok {
			c.log.With("identifier", identifier).Debug("verification failed: stream name is not a string", "stream", verified)

			return &common.CommandResult{
					Status:        common.FAILURE,
					Transmissions: []string{common.RejectionMessage(identifier)},
				},
				nil
		}

		c.log.With("identifier", identifier).Debug("verified", "stream", stream)
	}

	state := map[string]string{}

	if request.whisper {
		state[common.WHISPER_STREAM_STATE] = stream
	}

	if request.presence {
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

		if !allowPublic && request.StreamName != "" {
			return nil, errors.New("public streams are not allowed")
		}

		if whispers || (request.StreamName != "") {
			request.whisper = true
		}

		if presence || (request.StreamName != "") {
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
