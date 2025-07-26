package pusher

import (
	"errors"
	"strings"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
)

type Executor struct {
	app      node.Executor
	verifier *Verifier
}

var _ node.Executor = (*Executor)(nil)

func NewExecutor(app node.Executor, verifier *Verifier) *Executor {
	return &Executor{
		app:      app,
		verifier: verifier,
	}
}

func (ex *Executor) HandleCommand(s *node.Session, msg *common.Message) error {
	if msg.Command == "subscribe" {
		subscribeMsg := &common.Message{
			Command:    msg.Command,
			Identifier: msg.Identifier,
		}

		if err := ex.verifyPrivateChannel(s, msg); err != nil {
			s.Log.Debug("pusher authorization failed", "err", err)
			subscribeMsg.Identifier = channelToIdentifier("")
			return ex.app.HandleCommand(s, subscribeMsg)
		}

		return ex.app.HandleCommand(s, subscribeMsg)
	}

	return ex.app.HandleCommand(s, msg)
}

func (ex *Executor) Disconnect(s *node.Session) error {
	return ex.app.Disconnect(s)
}

func (ex *Executor) verifyPrivateChannel(s *node.Session, msg *common.Message) error {
	channel, err := identifierToChannel(msg.Identifier)
	if err != nil {
		return err
	}

	if strings.HasPrefix(channel, "private-") || strings.HasPrefix(channel, "presence-") {
		data, ok := msg.Data.(*PusherSubscriptionData)

		if !ok {
			return errors.New("missing auth data")
		}

		verified := false

		if strings.HasPrefix(channel, "presence-") {
			verified = ex.verifier.VerifyPresenceChannel(s.GetID(), channel, data.ChannelData, data.Auth)
			if verified {
				// This information will be used by the controller to trigger the presence join event
				// Unfortunately, there is no good way to pass through the original Pusher payload
				s.GetEnv().MergeChannelState(msg.Identifier, &map[string]string{"channel_data": data.ChannelData})
			}
		} else {
			verified = ex.verifier.VerifyChannel(s.GetID(), channel, data.Auth)
		}

		if !verified {
			return errors.New("invalid signature")
		}
	}

	return nil
}
