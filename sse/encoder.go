package sse

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/utils"
	"github.com/anycable/anycable-go/ws"
)

const sseEncoderID = "sse"

// Tell the client to reconnect in a year in case we don't really want it to re-connect
const retryNoReconnect = int64(31536000000)

const lastIdDelimeter = "/"

// Encoder is responsible for converting messages to SSE format (event:, data:, etc.)
// NOTE: It's only used to encode messages from server to client.
type Encoder struct {
	// Whether to send protocol events or just data messages
	RawData bool
	// Whether to send only the "message" field of the payload as data or the whole payload
	UnwrapData bool
}

func (Encoder) ID() string {
	return sseEncoderID
}

func (e *Encoder) Encode(msg encoders.EncodedMessage) (*ws.SentFrame, error) {
	msgType := msg.GetType()

	b, err := json.Marshal(&msg)
	if err != nil {
		panic("Failed to build JSON 😲")
	}

	var payload string

	reply, isReply := msg.(*common.Reply)

	if isReply && reply.Type == "" && e.UnwrapData {
		var data string

		if replyStr, ok := reply.Message.(string); ok {
			data = replyStr
		} else {
			data = string(utils.ToJSON(reply.Message))
		}
		payload = encodeSSEData(data)
	} else {
		payload = encodeSSEData(string(b))
	}

	if msgType != "" {
		if e.RawData {
			return nil, nil
		}

		payload = "event: " + msgType + "\n" + payload
	}

	if reply, ok := msg.(*common.Reply); ok {
		if reply.Offset > 0 && reply.Epoch != "" && reply.StreamID != "" {
			payload += "\nid: " + fmt.Sprintf("%d%s%s%s%s", reply.Offset, lastIdDelimeter, reply.Epoch, lastIdDelimeter, reply.StreamID)
		}
	}

	if msgType == "disconnect" {
		dmsg, ok := msg.(*common.DisconnectMessage)
		if ok && !dmsg.Reconnect {
			payload += "\nretry: " + fmt.Sprintf("%d", retryNoReconnect)
		}
	}

	return &ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte(payload)}, nil
}

// sseDataLineBreak rewrites every line terminator into a new `data:` field
// boundary. Order matters: CRLF is matched before lone CR/LF.
var sseDataLineBreak = strings.NewReplacer("\r\n", "\ndata: ", "\r", "\ndata: ", "\n", "\ndata: ")

// encodeSSEData formats a payload as one or more SSE `data:` fields.
//
// Per the SSE specification, an event's data is encoded as one `data:` field
// per line; the client (e.g. EventSource) rejoins the fields with "\n". A raw
// line terminator after a single `data:` prefix would truncate the payload at
// the first line break, because the following lines are parsed as fields other
// than `data` and ignored. LF, CR and CRLF are all line terminators per the
// spec, so all three are normalized. This matters for unwrapped payloads, which
// may be arbitrary multi-line strings; JSON-encoded payloads are unaffected
// (their newlines are already escaped).
func encodeSSEData(payload string) string {
	return "data: " + sseDataLineBreak.Replace(payload)
}

func (e Encoder) EncodeTransmission(raw string) (*ws.SentFrame, error) {
	msg := common.Reply{}

	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		return nil, err
	}

	return e.Encode(&msg)
}

func (Encoder) Decode(raw []byte) (*common.Message, error) {
	return nil, errors.New("unsupported")
}
