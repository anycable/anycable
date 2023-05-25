package rails

import (
	"encoding/json"

	"github.com/apex/log"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/utils"
)

type CableReadyController struct {
	verifier *utils.MessageVerifier
	log      *log.Entry
}

var _ node.Controller = (*CableReadyController)(nil)

func NewCableReadyController(key string) *CableReadyController {
	var verifier *utils.MessageVerifier

	if key != "" {
		verifier = utils.NewMessageVerifier(key)
	}

	return &CableReadyController{verifier, log.WithField("context", "cable_ready")}
}

func (c *CableReadyController) Start() error {
	return nil
}

func (c *CableReadyController) Shutdown() error {
	return nil
}

func (c *CableReadyController) Authenticate(sid string, env *common.SessionEnv) (*common.ConnectResult, error) {
	return nil, nil
}

func (c *CableReadyController) Subscribe(sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
	params := struct {
		SignedStreamID string `json:"identifier"`
	}{}

	err := json.Unmarshal([]byte(channel), &params)

	if err != nil {
		c.log.WithField("identifier", channel).Warnf("invalid identifier: %v", err)
		return nil, err
	}

	var stream string

	if c.IsCleartext() {
		stream = params.SignedStreamID

		c.log.WithField("identifier", channel).Debugf("unsigned stream: %s", stream)
	} else {
		verified, err := c.verifier.Verified(params.SignedStreamID)

		if err != nil {
			c.log.WithField("identifier", channel).Debugf("verification failed for %s: %v", params.SignedStreamID, err)

			return &common.CommandResult{
					Status:        common.FAILURE,
					Transmissions: []string{common.RejectionMessage(channel)},
				},
				nil
		}

		var ok bool

		stream, ok = verified.(string)

		if !ok {
			c.log.WithField("identifier", channel).Debugf("verification failed: stream name is not a string: %v", verified)

			return &common.CommandResult{
					Status:        common.FAILURE,
					Transmissions: []string{common.RejectionMessage(channel)},
				},
				nil
		}

		c.log.WithField("identifier", channel).Debugf("verified stream: %s", stream)
	}

	return &common.CommandResult{
		Status:             common.SUCCESS,
		Transmissions:      []string{common.ConfirmationMessage(channel)},
		Streams:            []string{stream},
		DisconnectInterest: -1,
	}, nil
}

func (c *CableReadyController) Unsubscribe(sid string, env *common.SessionEnv, id string, channel string) (*common.CommandResult, error) {
	return &common.CommandResult{
		Status:         common.SUCCESS,
		Transmissions:  []string{},
		Streams:        []string{},
		StopAllStreams: true,
	}, nil
}

func (c *CableReadyController) Perform(sid string, env *common.SessionEnv, id string, channel string, data string) (*common.CommandResult, error) {
	return nil, nil
}

func (c *CableReadyController) Disconnect(sid string, env *common.SessionEnv, id string, subscriptions []string) error {
	return nil
}

func (c *CableReadyController) IsCleartext() bool {
	return c.verifier == nil
}
