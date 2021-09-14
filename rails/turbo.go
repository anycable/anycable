package rails

import (
	"encoding/json"

	"github.com/apex/log"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/utils"
)

type TurboController struct {
	verifier *utils.MessageVerifier
	log      *log.Entry
}

var _ node.Controller = (*TurboController)(nil)

func NewTurboController(key string) *TurboController {
	verifier := utils.NewMessageVerifier(key)
	return &TurboController{verifier, log.WithField("context", "turbo")}
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
		SignedStreamId string `json:"signed_stream_name"`
	}{}

	err := json.Unmarshal([]byte(channel), &params)

	if err != nil {
		c.log.WithField("identifier", channel).Warnf("invalid identifier: %v", err)
		return nil, err
	}

	stream, err := c.verifier.Verified(params.SignedStreamId)

	if err != nil {
		c.log.WithField("identifier", channel).Debugf("verification failed for %s: %v", params.SignedStreamId, err)

		return &common.CommandResult{
				Status:        common.FAILURE,
				Transmissions: []string{common.RejectionMessage(channel)},
			},
			nil
	}

	c.log.WithField("identifier", channel).Debugf("verified stream: %s", stream)

	return &common.CommandResult{
		Status:        common.SUCCESS,
		Transmissions: []string{common.ConfirmationMessage(channel)},
		Streams:       []string{stream},
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
