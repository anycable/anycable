package rails

import (
	"encoding/json"
	"log/slog"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/utils"
)

type TurboController struct {
	verifier *utils.MessageVerifier
	log      *slog.Logger
}

var _ node.Controller = (*TurboController)(nil)

func NewTurboController(key string) *TurboController {
	var verifier *utils.MessageVerifier

	if key != "" {
		verifier = utils.NewMessageVerifier(key)
	}

	return &TurboController{verifier, slog.With("context", "turbo")}
}

func (c *TurboController) Start() error {
	return nil
}

func (c *TurboController) Shutdown() error {
	return nil
}

func (c *TurboController) Authenticate(sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	return nil, nil
}

func (c *TurboController) Subscribe(sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
	params := struct {
		SignedStreamID string `json:"signed_stream_name"`
	}{}

	err := json.Unmarshal([]byte(channel), &params)

	if err != nil {
		c.log.With("identifier", channel).Warn("invalid identifier", "error", err)
		return nil, err
	}

	var stream string

	if c.IsCleartext() {
		stream = params.SignedStreamID

		c.log.With("identifier", channel).Debug("unsigned", "stream", stream)
	} else {
		verified, err := c.verifier.Verified(params.SignedStreamID)

		if err != nil {
			c.log.With("identifier", channel).Debug("verification failed", "stream", params.SignedStreamID, "error", err)

			return &common.CommandResult{
					Status:        common.FAILURE,
					Transmissions: []string{common.RejectionMessage(channel)},
				},
				nil
		}

		var ok bool

		stream, ok = verified.(string)

		if !ok {
			c.log.With("identifier", channel).Debug("verification failed: stream name is not a string", "stream", verified)

			return &common.CommandResult{
					Status:        common.FAILURE,
					Transmissions: []string{common.RejectionMessage(channel)},
				},
				nil
		}

		c.log.With("identifier", channel).Debug("verified", "stream", stream)
	}

	return &common.CommandResult{
		Status:             common.SUCCESS,
		Transmissions:      []string{common.ConfirmationMessage(channel)},
		Streams:            []string{stream},
		DisconnectInterest: -1,
	}, nil
}

func (c *TurboController) Unsubscribe(sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
	return &common.CommandResult{
		Status:         common.SUCCESS,
		Transmissions:  []string{},
		Streams:        []string{},
		StopAllStreams: true,
	}, nil
}

func (c *TurboController) Perform(sid string, env *common.SessionEnv, id string, channel string, data string) (*common.CommandResult, error) {
	return nil, nil
}

func (c *TurboController) Disconnect(sid string, env *common.SessionEnv, id string, subscriptions []string) error {
	return nil
}

func (c *TurboController) IsCleartext() bool {
	return c.verifier == nil
}
