package ocpp

import (
	"errors"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/utils"
)

type PerformMessage struct {
	Action  string      `json:"action"`
	Command string      `json:"command"`
	ID      string      `json:"id"`
	Payload interface{} `json:"payload,omitempty"`
	Code    string      `json:"code,omitempty"`
	Message string      `json:"message,omitempty"`
}

// Executor handle incoming commands and client disconnections
type Executor struct {
	node node.AppNode
	conf *Config
}

var _ node.Executor = (*Executor)(nil)

func NewExecutor(node node.AppNode, c *Config) *Executor {
	return &Executor{node: node, conf: c}
}

func (ex *Executor) HandleCommand(s *node.Session, msg *common.Message) error {
	s.Log.Debug("incoming message", "data", msg)

	var sn string

	// Fast-track for heartbeats
	if msg.Command == HeartbeatCommand {
		s.Send(&common.Reply{
			Type:       AckCommand,
			Identifier: msg.Identifier,
			Message:    map[string]string{"currentTime": time.Now().Format(time.RFC3339)},
		})
		return nil
	}

	if msg.Command == BootCommand {
		if s.Connected {
			ex.sendError(s, msg.Identifier, FormationViolationError, "Already booted")
			return nil
		}

		bootPayload := msg.Data.(CallMessage).Payload

		if bootPayload == nil {
			ex.sendError(s, msg.Identifier, FormationViolationError, "Missing boot notification payload")
			return errors.New("missing boot notification payload")
		}

		if bootInfo, ok := bootPayload.(map[string]interface{}); ok {
			sn = bootInfo["meterSerialNumber"].(string)

			if sn == "" {
				ex.sendError(s, msg.Identifier, FormationViolationError, "Missing meter serial number")
				return errors.New("missing meter serial number")
			}

			s.WriteInternalState("sn", sn)
		} else {
			ex.sendError(s, msg.Identifier, FormationViolationError, "Invalid boot notification payload format")
			return errors.New("invalid boot notification payload format")
		}

		// First, perform authentication, then boot the client
		res, err := ex.node.Authenticate(s, node.WithDisconnectOnFailure(false))

		if err != nil {
			return err
		}

		// Authentication failed
		if res != nil && res.Status == common.FAILURE {
			// TODO: cooldown interval?
			ex.sendStatus(s, msg.Identifier, "Rejected", nil)
			ex.Disconnect(s) // nolint: errcheck
			return nil
		}

		channelId := IDToIdentifier(sn, ex.conf.ChannelName)

		// Now, subscribe to the channel
		subRes, err := ex.node.Subscribe(s,
			&common.Message{
				Identifier: channelId,
				Command:    "subscribe",
			},
		)

		if err != nil {
			ex.sendError(s, msg.Identifier, InternalError, "Application error")
			return err
		}

		// Subscription was rejected
		if subRes != nil && subRes.Status == common.FAILURE {
			s.Log.Debug("boot notification rejected")
			ex.sendStatus(s, msg.Identifier, "Rejected", nil)
			ex.Disconnect(s) // nolint: errcheck
			return nil
		}
	}

	if val, ok := s.ReadInternalState("sn"); ok {
		sn = val.(string)
	} else {
		ex.sendError(s, msg.Identifier, FormationViolationError, "No serial number")
		return errors.New("missing serial number in state")
	}

	// Regular RPC processing
	channelId := IDToIdentifier(sn, ex.conf.ChannelName)

	occpMsg, ok := msg.Data.(Message)

	if !ok {
		ex.sendError(s, msg.Identifier, FormationViolationError, "Unknown message format")
		return errors.New("unknown message format")
	}

	// Prepare perform payload by converting command to snake_case and performing and RPC call
	// to the application
	performPayload := PerformMessage{
		ID:      msg.Identifier,
		Command: msg.Command,
		Payload: occpMsg.GetPayload(),
	}

	if ex.conf.GranularActions {
		performPayload.Action = utils.ToSnakeCase(msg.Command)
	}

	if msg.Command == ErrorCommand {
		errPayload := msg.Data.(ErrorMessage)

		performPayload.Code = errPayload.ErrorCode
		performPayload.Message = errPayload.ErrorDescription
	}

	performMsg := common.Message{
		Command:    "message",
		Identifier: channelId,
		Data:       string(utils.ToJSON(performPayload)),
	}

	res, err := ex.node.Perform(s, &performMsg)

	if err != nil {
		ex.sendError(s, msg.Identifier, InternalError, "Application error")
		return err
	}

	// If no transmissions were returned,
	// send the ack ourselves (only for calls, not acks or errors)
	if occpMsg.GetCode() == CallCode && len(res.Transmissions) == 0 {
		var details map[string]interface{}

		// For boot notification, we need to send the heartbeat interval
		if msg.Command == BootCommand {
			details = map[string]interface{}{"interval": ex.conf.HeartbeatInterval}
		}

		ex.sendStatus(s, msg.Identifier, "Accepted", details)
	}

	return nil
}

func (ex *Executor) Disconnect(s *node.Session) error {
	// Do custom cleanup here
	return ex.node.Disconnect(s)
}

func (ex *Executor) sendError(s *node.Session, identifier string, code string, message string) {
	s.Send(&common.Reply{
		Type:       code,
		Identifier: identifier,
		Reason:     message,
	})
}

func (ex *Executor) sendStatus(s *node.Session, identifier string, status string, details map[string]interface{}) {
	if details == nil {
		details = make(map[string]interface{})
	}

	details["status"] = status

	s.Send(&common.Reply{
		Type:       AckCommand,
		Identifier: identifier,
		Message:    details,
	})
}

func IDToIdentifier(id string, channel string) string {
	msg := struct {
		Channel      string `json:"channel"`
		SerialNumber string `json:"sn"`
	}{Channel: channel, SerialNumber: id}

	return string(utils.ToJSON(msg))
}
