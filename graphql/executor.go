package graphql

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/node"
	"github.com/anycable/anycable-go/ws"
)

const (
	payloadHeader = "x-apollo-connection"
)

// Handling Apollo commands and transforming them into Action Cable commands
type Executor struct {
	node node.AppNode

	channel string
	action  string
}

var _ node.Executor = (*Executor)(nil)

func NewExecutor(node node.AppNode, config *Config) *Executor {
	return &Executor{node: node, channel: config.Channel, action: config.Action}
}

func (ex *Executor) HandleCommand(s *node.Session, msg *common.Message) error {
	s.Log.Debugf("Incoming message: %v", msg)

	if msg.Command == GQL_CONNECTION_INIT {
		if s.Connected {
			return errors.New("Already connected")
		}
		// Perform authentication
		// We automatically transform welcome message into connection ack,
		// so, no need to send it manually.
		// Also, we should pass payload, 'cause it may contain authentication data
		if msg.Data != nil {
			data, ok := msg.Data.(string)

			if !ok {
				return fmt.Errorf("GQL data must be a string, got %v", msg.Data)
			}

			if data != "" {
				s.GetEnv().SetHeader(payloadHeader, data)
			}
		}

		res, err := ex.node.Authenticate(s)

		if res != nil && res.Status == common.FAILURE {
			return nil
		}

		return err
	}

	if msg.Command == GQL_CONNECTION_TERMINATE {
		s.Disconnect("Terminate request", ws.CloseNormalClosure)
		return nil
	}

	if !s.Connected {
		return errors.New("Connection hasn't been initialized")
	}

	identifier := IDToIdentifier(msg.Identifier, ex.channel)

	if msg.Command == GQL_START {
		// 1. We need to perform two RPC calls: to create an Action Cable subscription,
		// and to execute a query.
		// 2. If query is not a subscription, we MUST remove the subscription and
		// send a complete message.
		_, err := ex.node.Subscribe(s, &common.Message{Identifier: identifier, Command: "subscribe"})

		if err != nil {
			return err
		}

		// OK, we subscribed, now let's execute the query.
		// First, deserialize the operation
		operation := GraphqlQuery{}

		data, ok := msg.Data.(string)

		if !ok {
			return fmt.Errorf("GQL data must be a string, got %v", msg.Data)
		}

		err = json.Unmarshal([]byte(data), &operation)

		if err != nil {
			return err
		}

		operation.Action = ex.action

		msg := common.Message{
			Command:    "message",
			Identifier: identifier,
			Data:       string(operation.ToJSON()),
		}

		s.Log.Debugf("Execute GraphQL query: %v", msg)

		res, err := ex.node.Perform(s, &msg)

		if err != nil {
			return err
		}

		// Now we need to check whether the query was subscription or not.
		// If not, we should remove it from the subscription lists.
		if len(res.Transmissions) != 1 {
			return fmt.Errorf("Expected query execution to return one transmission, got: %v", res)
		}

		reply := struct {
			Message struct {
				More bool `json:"more"`
			} `json:"message"`
		}{}

		err = json.Unmarshal([]byte(res.Transmissions[0]), &reply)

		if err != nil {
			return err
		}

		// This is subscription. We're done!
		if reply.Message.More {
			return nil
		}

		// This is not a subscription. Let's unsubscribe!
		return ex.completeRequest(s, identifier)
	}

	if msg.Command == GQL_STOP {
		return ex.completeRequest(s, identifier)
	}

	return fmt.Errorf("Unknown command: %s", msg.Command)
}

func (ex *Executor) Disconnect(s *node.Session) error {
	return ex.node.Disconnect(s)
}

func (ex *Executor) completeRequest(s *node.Session, identifier string) error {
	_, err := ex.node.Unsubscribe(s, &common.Message{Identifier: identifier, Command: "unsubscribe"})

	if err != nil {
		return err
	}

	s.Send(&common.Reply{Type: common.UnsubscribedType, Identifier: identifier})
	return nil
}
