package pusher

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/utils"
	"github.com/anycable/anycable-go/ws"
)

const encoderID = "pusher7"

const ChannelName = "$pusher"

// See https://pusher.com/docs/channels/library_auth_reference/pusher-websockets-protocol/
const (
	// Client sends this message to establish connection
	ConnectionEstablishedType = "pusher:connection_established"
	// Client sends this message to subscribe to a channel
	SubscribeType = "pusher:subscribe"
	// Server sends this message to confirm subscription
	SubscriptionSucceededType = "pusher_internal:subscription_succeeded"
	// Server sends this message when subscription fails
	SubscriptionErrorType = "pusher_internal:subscription_error"
	// Client sends this message to unsubscribe from a channel
	UnsubscribeType = "pusher:unsubscribe"
	// Server sends this message with channel data
	EventType = "pusher:event"
	// Client events (whispers) start with the "client-" prefix
	ClientTypePrefix = "client-"
	// Ping/pong for keepalive
	PingType = "pusher:ping"
	PongType = "pusher:pong"
	// Presence messages
	MemberAddedType   = "pusher_internal:member_added"
	MemberRemovedType = "pusher_internal:member_removed"
	// Error message
	ErrorType = "pusher:error"
)

const PingPayload = "{\"event\":\"pusher:ping\",\"data\":{}}"
const PongPayload = "{\"event\":\"pusher:pong\"}"

type PusherMessage struct {
	Event   string      `json:"event"`
	Data    interface{} `json:"data,omitempty"`
	Channel string      `json:"channel,omitempty"`
}

type PusherSubscriptionData struct {
	Channel     string `json:"channel"`
	Auth        string `json:"auth,omitempty"`
	ChannelData string `json:"channel_data,omitempty"`
}

type PusherSubscriptionEvent struct {
	Event string                  `json:"event"`
	Data  *PusherSubscriptionData `json:"data"`
}

type PusherClientEvent struct {
	Event   string      `json:"event"`
	Channel string      `json:"channel"`
	Data    interface{} `json:"data"`
}

type PusherConnectionData struct {
	SocketID        string `json:"socket_id"`
	ActivityTimeout int    `json:"activity_timeout"`
}

type PusherPresenceData struct {
	UserId   string      `json:"user_id"`
	UserInfo interface{} `json:"user_info,omitempty"`
}

type errorCode = int

const (
	// reconnect: false
	errorUnauthorized errorCode = 4009
	// reconnect: true
	errorGeneric errorCode = 4200
)

type PusherErrorData struct {
	Message string    `json:"message"`
	Code    errorCode `json:"code"`
}

func (msg *PusherMessage) DataString() (string, error) {
	if msg.Data == nil {
		return "{}", nil
	}

	if str, ok := msg.Data.(string); ok {
		return str, nil
	}

	jsonStr, err := json.Marshal(msg.Data)
	if err != nil {
		return "", err
	}
	return string(jsonStr), nil
}

type Encoder struct {
	ActivityTimeout int
}

func NewEncoder() *Encoder {
	return &Encoder{ActivityTimeout: 30}
}

var _ encoders.Encoder = (*Encoder)(nil)

func (*Encoder) ID() string {
	return encoderID
}

func (pusher *Encoder) Encode(msg encoders.EncodedMessage) (*ws.SentFrame, error) {
	mtype := msg.GetType()

	if mtype == common.PingType {
		return &ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte(PingPayload)}, nil
	}

	if mtype == common.PongType {
		return &ws.SentFrame{FrameType: ws.TextFrame, Payload: []byte(PongPayload)}, nil
	}

	if mtype == common.DisconnectType {
		dm := msg.(*common.DisconnectMessage)

		errorMsg := PusherErrorData{Message: dm.Reason}

		if dm.Reconnect {
			errorMsg.Code = errorGeneric
		} else {
			errorMsg.Code = errorUnauthorized
		}

		return &ws.SentFrame{FrameType: ws.TextFrame, Payload: utils.ToJSON(PusherMessage{Event: ErrorType, Data: errorMsg})}, nil
	}

	r, ok := msg.(*common.Reply)

	if !ok {
		return nil, fmt.Errorf("unknown message type: %v", msg)
	}

	if r.Type == common.ConfirmedType {
		channel, err := identifierToChannel(r.Identifier)
		if err != nil {
			return nil, err
		}

		var data interface{}
		data = "{}" // empty JSON
		if r.Message != nil {
			data = r.Message
		}

		pusherMsg := PusherMessage{
			Event:   SubscriptionSucceededType,
			Channel: channel,
			Data:    data,
		}
		return &ws.SentFrame{FrameType: ws.TextFrame, Payload: utils.ToJSON(pusherMsg)}, nil
	}

	if r.Type == common.RejectedType {
		errorMsg := PusherErrorData{Message: "rejected", Code: errorUnauthorized}

		return &ws.SentFrame{FrameType: ws.TextFrame, Payload: utils.ToJSON(PusherMessage{Event: ErrorType, Data: errorMsg})}, nil
	}

	if r.Type == common.WelcomeType {
		connectionData := PusherConnectionData{
			SocketID:        r.Sid,
			ActivityTimeout: pusher.ActivityTimeout,
		}
		pusherMsg := PusherMessage{
			Event: ConnectionEstablishedType,
			Data:  connectionData,
		}
		return &ws.SentFrame{FrameType: ws.TextFrame, Payload: utils.ToJSON(pusherMsg)}, nil
	}

	if r.Type == common.PresenceType {
		presenceEvent, ok := r.Message.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("failed to encode presence event: %v", r.Message)
		}

		presenceData := PusherPresenceData{
			UserId: presenceEvent["id"].(string),
		}

		var pusherEvent string

		presenceType := presenceEvent["type"].(string)

		if presenceType == common.PresenceJoinType {
			pusherEvent = MemberAddedType
			presenceData.UserInfo = presenceEvent["info"]
		} else if presenceType == common.PresenceLeaveType {
			pusherEvent = MemberRemovedType
		}

		channel, err := identifierToChannel(r.Identifier)
		if err != nil {
			return nil, err
		}

		pusherMsg := PusherMessage{
			Event:   pusherEvent,
			Data:    string(utils.ToJSON(presenceData)),
			Channel: channel,
		}

		return &ws.SentFrame{FrameType: ws.TextFrame, Payload: utils.ToJSON(pusherMsg)}, nil
	}

	channel, err := identifierToChannel(r.Identifier)
	if err != nil {
		return nil, err
	}

	eventName := ""
	eventData := r.Message

	if m, ok := r.Message.(map[string]interface{}); ok {
		if event, exists := m["event"]; exists {
			if eventStr, ok := event.(string); ok {
				eventName = eventStr
			}
		}
		if data, exists := m["data"]; exists {
			eventData = data
		}
	}

	if eventName == "" {
		return nil, nil
	}

	pusherMsg := PusherMessage{
		Event:   eventName,
		Channel: channel,
		Data:    eventData,
	}

	return &ws.SentFrame{FrameType: ws.TextFrame, Payload: utils.ToJSON(pusherMsg)}, nil
}

func (pusher *Encoder) EncodeTransmission(raw string) (*ws.SentFrame, error) {
	msg := common.Reply{}

	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		return nil, err
	}

	return pusher.Encode(&msg)
}

func (pusher *Encoder) Decode(raw []byte) (*common.Message, error) {
	pusherMsg := &PusherMessage{}

	if err := json.Unmarshal(raw, pusherMsg); err != nil {
		return nil, err
	}

	msg := &common.Message{}

	if pusherMsg.Event == SubscribeType || pusherMsg.Event == UnsubscribeType {
		subEvent := &PusherSubscriptionEvent{}
		if err := json.Unmarshal(raw, subEvent); err != nil {
			return nil, err
		}
		msg.Identifier = channelToIdentifier(subEvent.Data.Channel)

		if pusherMsg.Event == SubscribeType {
			msg.Command = "subscribe"
		} else {
			msg.Command = "unsubscribe"
		}

		if subEvent.Data != nil {
			msg.Data = subEvent.Data
		}
	}

	if strings.HasPrefix(pusherMsg.Event, "client-") {
		clientEvent := &PusherClientEvent{}
		if err := json.Unmarshal(raw, clientEvent); err != nil {
			return nil, err
		}
		msg.Identifier = channelToIdentifier(clientEvent.Channel)
		msg.Command = "whisper"
		msg.Data = clientEvent
	}

	if pusherMsg.Event == PingType {
		msg.Command = "ping"
	}

	if pusherMsg.Event == PongType {
		msg.Command = "pong"
	}

	if msg.Command == "" {
		return nil, fmt.Errorf("unsupported event: %s", pusherMsg.Event)
	}

	return msg, nil
}

func identifierToChannel(id string) (string, error) {
	msg := struct {
		Channel string `json:"channel"`
		Stream  string `json:"stream"`
	}{}

	if err := json.Unmarshal([]byte(id), &msg); err != nil {
		return "", err
	}

	if msg.Channel != ChannelName {
		return "", fmt.Errorf("invalid channel name: %s", msg.Channel)
	}

	return msg.Stream, nil
}

func channelToIdentifier(channel string) string {
	msg := struct {
		Channel    string `json:"channel"`
		StreamName string `json:"stream"`
	}{Channel: ChannelName, StreamName: channel}

	return string(utils.ToJSON(msg))
}
