package ocpp

import (
	"encoding/json"
	"fmt"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/utils"
	"github.com/anycable/anycable-go/ws"
	"github.com/joomcode/errorx"

	nanoid "github.com/matoous/go-nanoid"
)

// Encoder converts messages from/to OCPP format to AnyCable format
type Encoder struct {
}

var _ encoders.Encoder = (*Encoder)(nil)

const ocppEncoderID = "ocpp"

func (Encoder) ID() string {
	return ocppEncoderID
}

func (enc Encoder) Encode(msg encoders.EncodedMessage) (*ws.SentFrame, error) {
	mtype := msg.GetType()

	// Ignore pings, disconnects, welcome messages
	if mtype == common.PingType || mtype == common.DisconnectType {
		return nil, nil
	}

	r, ok := msg.(*common.Reply)

	if !ok {
		return nil, fmt.Errorf("unknown message type: %v", msg)
	}

	// We handle subscription confirmation and rejection in the executor
	if r.Type == common.WelcomeType || r.Type == common.ConfirmedType || r.Type == common.RejectedType {
		return nil, nil
	}

	// No type -> broadasted message
	if mtype == "" {
		return enc.EncodeTransmission(string(utils.ToJSON(msg)))
	}

	var response interface{}

	if r.Type == AckCommand {
		response = [3]interface{}{
			AckCode,
			r.Identifier,
			r.Message,
		}
	} else {
		if r.Reason != "" {
			response = [5]interface{}{
				ErrorCode,
				r.Identifier,
				r.Type,
				r.Reason,
				r.Message,
			}
		} else {
			response = [4]interface{}{
				CallCode,
				r.Identifier,
				r.Type,
				r.Message,
			}
		}
	}

	b, err := json.Marshal(response)

	if err != nil {
		return nil, err
	}

	return &ws.SentFrame{FrameType: ws.TextFrame, Payload: b}, nil
}

type transmission struct {
	Command      string      `json:"command"`
	ID           string      `json:"id,omitempty"`
	Payload      interface{} `json:"payload,omitempty"`
	ErrorCode    string      `json:"error_code,omitempty"`
	ErrorMessage string      `json:"error_message,omitempty"`
}

// A copy of common.Reply with Message as json.RawMessage
type transmissionReply struct {
	Type       string          `json:"type,omitempty"`
	Identifier string          `json:"identifier,omitempty"`
	Message    json.RawMessage `json:"message,omitempty"`
	Reason     string          `json:"reason,omitempty"`
	Reconnect  bool            `json:"reconnect,omitempty"`
}

func (t transmissionReply) toReply() common.Reply {
	return common.Reply{
		Type:       t.Type,
		Identifier: t.Identifier,
		Message:    t.Message,
		Reason:     t.Reason,
		Reconnect:  t.Reconnect,
	}
}

// EncodeTransmission converts Action Cable message to OCPP format.
// Action Cable message contains a JSON-encoded string with the following structure:
//   - command: OCPP command or "Ack" or "Error" for specific commands
//   - id: Unique message ID (or reply ID)
//   - payload: Message payload
//   - error_code: Error code (for "Error" command)
//   - error_message: Error description (for "Error" command)
func (enc Encoder) EncodeTransmission(raw string) (*ws.SentFrame, error) {
	msg := transmissionReply{}

	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		return nil, err
	}

	payload := transmission{}

	if msg.Message != nil {
		if err := json.Unmarshal(msg.Message, &payload); err != nil {
			return nil, errorx.Decorate(err, "failed to decode message payload: %v", msg.Message)
		}

		if payload.ID != "" {
			msg.Identifier = payload.ID
		} else {
			uuid, err := nanoid.Nanoid()

			if err != nil {
				return nil, errorx.Decorate(err, "Failed to generate message ID")
			}
			// Generate a random ID with nanoid
			msg.Identifier = uuid
		}

		if payload.Command != "" {
			msg.Type = payload.Command
		} else {
			return nil, fmt.Errorf("missing command in message payload: %v", msg.Message)
		}

		if payload.Command == ErrorCommand {
			msg.Type = payload.ErrorCode
			msg.Reason = payload.ErrorMessage
		}
	}

	reply := msg.toReply()
	reply.Message = payload.Payload

	return enc.Encode(&reply)
}

func (Encoder) Decode(raw []byte) (*common.Message, error) {
	rawMsg := []interface{}{}

	if err := json.Unmarshal(raw, &rawMsg); err != nil {
		return nil, err
	}

	msgCodeFloat, ok := rawMsg[0].(float64)

	msgCode := int(msgCodeFloat)

	if !ok {
		return nil, fmt.Errorf("unknown message code format: %v", rawMsg[0])
	}

	id, _ := rawMsg[1].(string)

	var command string
	var payload interface{}

	switch msgCode {
	case AckCode:
		payload = AckMessage{UniqID: id, Payload: rawMsg[2]}
		command = AckCommand
	case CallCode:
		command = rawMsg[2].(string)
		payload = CallMessage{UniqID: id, Payload: rawMsg[3], Command: command}
	case ErrorCode:
		command = ErrorCommand
		errMsg := ErrorMessage{UniqID: id}

		if len(rawMsg) > 2 {
			errMsg.ErrorCode = rawMsg[2].(string)
		}

		if len(rawMsg) > 3 {
			errMsg.ErrorDescription = rawMsg[3].(string)
		}

		if len(rawMsg) > 4 {
			errMsg.Payload = rawMsg[4].(map[string]interface{})
		}

		payload = errMsg
	default:
		return nil, fmt.Errorf("unknown message type: %v", rawMsg)
	}

	msg := common.Message{Command: command, Identifier: id, Data: payload}

	return &msg, nil
}
