package apollo

import (
	"encoding/json"
	"fmt"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/ws"
)

const apolloEncoderId = "agql"

const (
	// Client sends this message after plain websocket connection to start the communication with the server
	GQL_CONNECTION_INIT = "connection_init"
	// The server may responses with this message to the GQL_CONNECTION_INIT from client, indicates the server rejected the connection.
	GQL_CONNECTION_ERROR = "connection_error"
	// Client sends this message to execute GraphQL operation
	GQL_START = "start"
	// Client sends this message in order to stop a running GraphQL operation execution (for example: unsubscribe)
	GQL_STOP = "stop"
	// Server sends this message upon a failing operation, before the GraphQL execution, usually due to GraphQL validation errors (resolver errors are part of GQL_DATA message, and will be added as errors array)
	GQL_ERROR = "error"
	// The server sends this message to transfter the GraphQL execution result from the server to the client, this message is a response for GQL_START message.
	GQL_DATA = "data"
	// Server sends this message to indicate that a GraphQL operation is done, and no more data will arrive for the specific operation.
	GQL_COMPLETE = "complete"
	// Server message that should be sent right after each GQL_CONNECTION_ACK processed and then periodically to keep the client connection alive.
	// The client starts to consider the keep alive message only upon the first received keep alive message from the server.
	GQL_CONNECTION_KEEP_ALIVE = "ka"
	// The server may responses with this message to the GQL_CONNECTION_INIT from client, indicates the server accepted the connection. May optionally include a payload.
	GQL_CONNECTION_ACK = "connection_ack"
	// Client sends this message to terminate the connection.
	GQL_CONNECTION_TERMINATE = "connection_terminate"
	// Unknown operation type, for logging only
	GQL_UNKNOWN = "unknown"
	// Internal status, for logging only
	GQL_INTERNAL = "internal"
)

const PingPayload = "{\"type\":\"ka\"}"
const DisconnectPayload = "{\"type\":\"connection_error\"}"

type GraphqlOperation struct {
	Id      string      `json:"id,omitempty"`
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

func (op *GraphqlOperation) ToJSON() []byte {
	jsonStr, err := json.Marshal(&op)
	if err != nil {
		panic("Failed to build GraphQL JSON 😲")
	}
	return jsonStr
}

type GraphqlQuery struct {
	Query         string      `json:"query"`
	Variables     interface{} `json:"variables,omitempty"`
	OperationName string      `json:"operationName,omitempty"`
	OperationId   string      `json:"operationId,omitempty"`
	// Required for Action Cable compatibility
	Action string `json:"action,omitempty"`
}

func (op *GraphqlQuery) ToJSON() []byte {
	jsonStr, err := json.Marshal(&op)
	if err != nil {
		panic("Failed to build GraphQL JSON 😲")
	}
	return jsonStr
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

func (Encoder) ID() string {
	return apolloEncoderId
}

func (gql Encoder) Encode(msg encoders.EncodedMessage) (*ws.SentFrame, error) {
	mtype := msg.GetType()

	if mtype == common.PingType {
		return &ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte(PingPayload)}, nil
	}

	if mtype == common.DisconnectType {
		return &ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte(DisconnectPayload)}, nil
	}

	r, ok := msg.(*common.Reply)

	if !ok {
		return nil, fmt.Errorf("Unknown message type: %v", msg)
	}

	if r.Type == common.ConfirmedType {
		return nil, nil
	}

	if r.Type == common.WelcomeType {
		operation := GraphqlOperation{Type: GQL_CONNECTION_ACK}
		return &ws.SentFrame{FrameType: ws.TextFrame, Payload: operation.ToJSON()}, nil
	}

	id, err := IdentifierToId(r.Identifier)

	if err != nil {
		return nil, err
	}

	operation := GraphqlOperation{Id: id, Payload: r.Message}

	if r.Type == common.RejectedType {
		operation.Type = GQL_ERROR
	} else if r.Type == common.UnsubscribedType {
		operation.Type = GQL_COMPLETE
	} else {
		operation.Type = GQL_DATA

		// GraphqlChannel responds with `{result: ..., more:...}` but
		// we only need the result here
		if m, ok := operation.Payload.((map[string]interface{})); ok {
			if r, k := m["result"]; k {
				operation.Payload = r
			}
		}
	}

	return &ws.SentFrame{FrameType: ws.TextFrame, Payload: operation.ToJSON()}, nil
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

	msg := &common.Message{Command: string(operation.Type)}

	if operation.Type == GQL_CONNECTION_INIT {
		payload, err := operation.PayloadString()

		if err != nil {
			return nil, err
		}

		msg.Data = payload
		return msg, nil
	}

	if operation.Type == GQL_CONNECTION_TERMINATE {
		return msg, nil
	}

	// Start and stop commands must include ID
	if operation.Type == GQL_START || operation.Type == GQL_STOP {
		msg.Identifier = operation.Id
	}

	// Start command also contains the query
	if operation.Type == GQL_START {
		payload, err := operation.PayloadString()

		if err != nil {
			return nil, err
		}

		msg.Data = payload
	}

	return msg, nil
}

var _ encoders.Encoder = (*Encoder)(nil)

func IdentifierToId(id string) (string, error) {
	msg := struct {
		ID string `json:"channelID"`
	}{}

	if err := json.Unmarshal([]byte(id), &msg); err != nil {
		return "", err
	}

	return msg.ID, nil
}

func IdToIdentifier(id string, channel string) string {
	msg := struct {
		Channel   string `json:"channel"`
		ChannelID string `json:"channelId"`
	}{Channel: channel, ChannelID: id}

	b, err := json.Marshal(msg)

	if err != nil {
		panic("Failed to build GraphQL identifier 😲")
	}

	return string(b)
}
