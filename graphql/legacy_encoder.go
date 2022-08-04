package graphql

import (
	"encoding/json"
	"fmt"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/utils"
	"github.com/anycable/anycable-go/ws"
)

const legacyEncoderID = "agql"

// See https://github.com/apollographql/subscriptions-transport-ws/blob/master/PROTOCOL.md#graphql-over-websocket-protocol
// nolint
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

const LegacyPingPayload = "{\"type\":\"ka\"}"
const DisconnectPayload = "{\"type\":\"connection_error\"}"

type LegacyEncoder struct {
}

var _ encoders.Encoder = (*LegacyEncoder)(nil)

func (LegacyEncoder) ID() string {
	return legacyEncoderID
}

func (gql LegacyEncoder) Encode(msg encoders.EncodedMessage) (*ws.SentFrame, error) {
	mtype := msg.GetType()

	if mtype == common.PingType {
		return &ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte(LegacyPingPayload)}, nil
	}

	if mtype == common.DisconnectType {
		return &ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte(DisconnectPayload)}, nil
	}

	r, ok := msg.(*common.Reply)

	if !ok {
		return nil, fmt.Errorf("unknown message type: %v", msg)
	}

	if r.Type == common.ConfirmedType {
		return nil, nil
	}

	if r.Type == common.WelcomeType {
		operation := GraphqlOperation{Type: GQL_CONNECTION_ACK}
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
			operation.Type = GQL_ERROR
		}
	case common.UnsubscribedType:
		{
			operation.Type = GQL_COMPLETE
		}
	default:
		{
			operation.Type = GQL_DATA

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

func (gql LegacyEncoder) EncodeTransmission(raw string) (*ws.SentFrame, error) {
	msg := common.Reply{}

	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		return nil, err
	}

	return gql.Encode(&msg)
}

func (gql LegacyEncoder) Decode(raw []byte) (*common.Message, error) {
	operation := &GraphqlOperation{}

	if err := json.Unmarshal(raw, &operation); err != nil {
		return nil, err
	}

	msg := &common.Message{Command: operation.Type}

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
		msg.Identifier = operation.ID
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
