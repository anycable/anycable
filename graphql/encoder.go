package graphql

import (
	"encoding/json"
	"fmt"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/utils"
	"github.com/anycable/anycable-go/ws"
)

const encoderID = "gql"

// See https://github.com/enisdenjo/graphql-ws/blob/master/PROTOCOL.md
const (
	// Client sends this message after plain websocket connection to start the communication with the server
	ConnectionInitType = "connection_init"
	// The server may responses with this message to the connection_init from client, indicates the server accepted the connection. May optionally include a payload.
	ConnectionAckType = "connection_ack"
	PingType          = "ping"
	PongType          = "pong"
	// Client sends this message to execute GraphQL operation
	SubscribeType = "subscribe"
	// The server sends this message to transfter the GraphQL execution result from the server to the client, this message is a response for subscribe message.
	NextType = "next"
	// Server sends this message upon a failing operation, before the GraphQL execution, usually due to GraphQL validation errors (resolver errors are part of next message, and will be added as errors array)
	ErrorType = "error"
	// Server sends this message to indicate that a GraphQL operation is done, and no more data will arrive for the specific operation.
	CompleteType = "complete"
)

const PingPayload = "{\"type\":\"ping\"}"
const PongPayload = "{\"type\":\"pong\"}"

type GraphqlOperation struct {
	ID      string      `json:"id,omitempty"`
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

type GraphqlQuery struct {
	Query         string      `json:"query"`
	Variables     interface{} `json:"variables,omitempty"`
	Extensions    interface{} `json:"extensions,omitempty"`
	OperationName string      `json:"operationName,omitempty"`
	OperationID   string      `json:"operationId,omitempty"`
	// Required for Action Cable compatibility
	Action string `json:"action,omitempty"`
}

func (op *GraphqlOperation) PayloadString() (string, error) {
	jsonStr, err := json.Marshal(&op.Payload)
	if err != nil {
		return "", err
	}
	return string(jsonStr), nil
}

type Encoder struct {
}

var _ encoders.Encoder = (*Encoder)(nil)

func (Encoder) ID() string {
	return encoderID
}

func (gql Encoder) Encode(msg encoders.EncodedMessage) (*ws.SentFrame, error) {
	mtype := msg.GetType()

	if mtype == common.PingType {
		return &ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte(PingPayload)}, nil
	}

	if mtype == PongType {
		return &ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte(PongPayload)}, nil
	}

	if mtype == common.DisconnectType {
		return nil, nil
	}

	r, ok := msg.(*common.Reply)

	if !ok {
		return nil, fmt.Errorf("unknown message type: %v", msg)
	}

	if r.Type == common.ConfirmedType {
		return nil, nil
	}

	if r.Type == common.WelcomeType {
		operation := GraphqlOperation{Type: ConnectionAckType}
		return &ws.SentFrame{FrameType: ws.TextFrame, Payload: utils.ToJSON(operation)}, nil
	}

	id, err := IdentifierToID(r.Identifier)

	if err != nil {
		return nil, err
	}

	operation := GraphqlOperation{ID: id, Payload: r.Message}

	switch r.Type {
	case common.RejectedType:
		{
			operation.Type = ErrorType
		}
	case common.UnsubscribedType:
		{
			operation.Type = CompleteType
		}
	default:
		{
			operation.Type = NextType

			// GraphqlChannel responds with `{result: ..., more:...}` but
			// we only need the result here
			if m, ok := operation.Payload.((map[string]interface{})); ok {
				if r, k := m["result"]; k {
					operation.Payload = r
				}
			}
		}
	}

	return &ws.SentFrame{FrameType: ws.TextFrame, Payload: utils.ToJSON(operation)}, nil
}

func (gql Encoder) EncodeTransmission(raw string) (*ws.SentFrame, error) {
	msg := common.Reply{}

	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		return nil, err
	}

	return gql.Encode(&msg)
}

func (gql Encoder) Decode(raw []byte) (*common.Message, error) {
	operation := &GraphqlOperation{}

	if err := json.Unmarshal(raw, &operation); err != nil {
		return nil, err
	}

	msg := &common.Message{Command: operation.Type}

	if operation.Type == ConnectionInitType {
		payload, err := operation.PayloadString()

		if err != nil {
			return nil, err
		}

		msg.Data = payload
		return msg, nil
	}

	// Start and stop commands must include ID
	if operation.Type == SubscribeType || operation.Type == CompleteType {
		msg.Identifier = operation.ID
	}

	// Start command also contains the query
	if operation.Type == SubscribeType {
		payload, err := operation.PayloadString()

		if err != nil {
			return nil, err
		}

		msg.Data = payload
	}

	return msg, nil
}

func IdentifierToID(id string) (string, error) {
	msg := struct {
		ID string `json:"channelID"`
	}{}

	if err := json.Unmarshal([]byte(id), &msg); err != nil {
		return "", err
	}

	return msg.ID, nil
}

func IDToIdentifier(id string, channel string) string {
	msg := struct {
		Channel   string `json:"channel"`
		ChannelID string `json:"channelId"`
	}{Channel: channel, ChannelID: id}

	return string(utils.ToJSON(msg))
}
