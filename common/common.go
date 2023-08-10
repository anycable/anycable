// Package common contains struts and interfaces shared between multiple components
package common

import (
	"encoding/json"
)

// Command result status
const (
	SUCCESS = iota
	FAILURE
	ERROR
)

const (
	ActionCableV1JSON    = "actioncable-v1-json"
	ActionCableV1ExtJSON = "actioncable-v1-ext-json"
)

func ActionCableProtocols() []string {
	return []string{ActionCableV1JSON, ActionCableV1ExtJSON}
}

func ActionCableExtendedProtocols() []string {
	return []string{ActionCableV1ExtJSON}
}

func IsExtendedActionCableProtocol(protocol string) bool {
	for _, p := range ActionCableExtendedProtocols() {
		if p == protocol {
			return true
		}
	}

	return false
}

// Outgoing message types (according to Action Cable protocol)
const (
	WelcomeType    = "welcome"
	PingType       = "ping"
	DisconnectType = "disconnect"
	ConfirmedType  = "confirm_subscription"
	RejectedType   = "reject_subscription"
	// Not supported by Action Cable currently
	UnsubscribedType = "unsubscribed"

	HistoryConfirmedType = "confirm_history"
	HistoryRejectedType  = "reject_history"
)

// Disconnect reasons
const (
	SERVER_RESTART_REASON    = "server_restart"
	REMOTE_DISCONNECT_REASON = "remote"
	IDLE_TIMEOUT_REASON      = "idle_timeout"
	UNAUTHORIZED_REASON      = "unauthorized"
)

// SessionEnv represents the underlying HTTP connection data:
// URL and request headers.
// It also carries channel and connection state information used by the RPC app.
type SessionEnv struct {
	URL             string
	Headers         *map[string]string
	Identifiers     string
	ConnectionState *map[string]string
	ChannelStates   *map[string]map[string]string
}

// NewSessionEnv builds a new SessionEnv
func NewSessionEnv(url string, headers *map[string]string) *SessionEnv {
	state := make(map[string]string)
	channels := make(map[string]map[string]string)
	return &SessionEnv{
		URL:             url,
		Headers:         headers,
		ConnectionState: &state,
		ChannelStates:   &channels,
	}
}

// MergeConnectionState updates the current ConnectionState from the given map.
// If the value is an empty string then remove the key,
// otherswise add or rewrite.
func (st *SessionEnv) MergeConnectionState(other *map[string]string) {
	for k, v := range *other {
		if v == "" {
			delete(*st.ConnectionState, k)
		} else {
			(*st.ConnectionState)[k] = v
		}
	}
}

// MergeChannelState updates the current ChannelStates for the given identifier.
// If the value is an empty string then remove the key,
// otherswise add or rewrite.
func (st *SessionEnv) MergeChannelState(id string, other *map[string]string) {
	if _, ok := (*st.ChannelStates)[id]; !ok {
		(*st.ChannelStates)[id] = make(map[string]string)
	}

	for k, v := range *other {
		if v == "" {
			delete((*st.ChannelStates)[id], k)
		} else {
			(*st.ChannelStates)[id][k] = v
		}
	}
}

// Returns a value for the specified key of the specified channel
func (st *SessionEnv) GetChannelStateField(id string, field string) string {
	cst, ok := (*st.ChannelStates)[id]

	if !ok {
		return ""
	}

	return cst[field]
}

// Returns a value for the specified connection state field
func (st *SessionEnv) GetConnectionStateField(field string) string {
	if st.ConnectionState == nil {
		return ""
	}

	return (*st.ConnectionState)[field]
}

// SetHeader adds a header to the headers list
func (st *SessionEnv) SetHeader(key string, val string) {
	if st.Headers == nil {
		headers := map[string]string{key: val}
		st.Headers = &headers
		return
	}

	(*st.Headers)[key] = val
}

// CallResult contains shared RPC result fields
type CallResult struct {
	Transmissions []string
	Broadcasts    []*StreamMessage
	CState        map[string]string
	IState        map[string]string
}

// ConnectResult is a result of initializing a connection (calling a Connect method)
type ConnectResult struct {
	Identifier         string
	Transmissions      []string
	Broadcasts         []*StreamMessage
	CState             map[string]string
	IState             map[string]string
	DisconnectInterest int
	Status             int
}

// ToCallResult returns the corresponding CallResult
func (c *ConnectResult) ToCallResult() *CallResult {
	res := CallResult{Transmissions: c.Transmissions, Broadcasts: c.Broadcasts}
	if c.CState != nil {
		res.CState = c.CState
	}
	if c.IState != nil {
		res.IState = c.IState
	}
	return &res
}

// CommandResult is a result of performing controller action,
// which contains informations about streams to subscribe,
// messages to sent and broadcast.
// It's a communication "protocol" between a node and a controller.
type CommandResult struct {
	StopAllStreams     bool
	Disconnect         bool
	Streams            []string
	StoppedStreams     []string
	Transmissions      []string
	Broadcasts         []*StreamMessage
	CState             map[string]string
	IState             map[string]string
	DisconnectInterest int
	Status             int
}

// ToCallResult returns the corresponding CallResult
func (c *CommandResult) ToCallResult() *CallResult {
	res := CallResult{Transmissions: c.Transmissions, Broadcasts: c.Broadcasts}
	if c.CState != nil {
		res.CState = c.CState
	}
	if c.IState != nil {
		res.IState = c.IState
	}
	return &res
}

type HistoryPosition struct {
	Epoch  string `json:"epoch"`
	Offset uint64 `json:"offset"`
}

// HistoryRequest represents a client's streams state (offsets) or a timestamp since
// which we should return the messages for the current streams
type HistoryRequest struct {
	// Since is UTC timestamp in ms
	Since int64 `json:"since,omitempty"`
	// Streams contains the information of last offsets/epoch received for a particular stream
	Streams map[string]HistoryPosition `json:"streams,omitempty"`
}

// Message represents incoming client message
type Message struct {
	Command    string         `json:"command"`
	Identifier string         `json:"identifier"`
	Data       interface{}    `json:"data,omitempty"`
	History    HistoryRequest `json:"history,omitempty"`
}

// StreamMessage represents a pub/sub message to be sent to stream
type StreamMessage struct {
	Stream string `json:"stream"`
	Data   string `json:"data"`

	// Offset is the position of this message in the stream
	Offset uint64
	// Epoch is the uniq ID of the current storage state
	Epoch string
}

func (sm *StreamMessage) ToReplyFor(identifier string) *Reply {
	data := sm.Data

	var msg interface{}

	// We ignore JSON deserialization failures and consider the message to be a string
	json.Unmarshal([]byte(data), &msg) // nolint:errcheck

	if msg == nil {
		msg = sm.Data
	}

	stream := ""

	// Only include stream if offset/epovh is present
	if sm.Epoch != "" {
		stream = sm.Stream
	}

	return &Reply{
		Identifier: identifier,
		Message:    msg,
		StreamID:   stream,
		Offset:     sm.Offset,
		Epoch:      sm.Epoch,
	}
}

// RemoteCommandMessage represents a pub/sub message with a remote command (e.g., disconnect)
type RemoteCommandMessage struct {
	Command string          `json:"command,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func (m *RemoteCommandMessage) ToRemoteDisconnectMessage() (*RemoteDisconnectMessage, error) {
	dmsg := RemoteDisconnectMessage{}

	if err := json.Unmarshal(m.Payload, &dmsg); err != nil {
		return nil, err
	}

	return &dmsg, nil
}

// RemoteDisconnectMessage contains information required to disconnect a session
type RemoteDisconnectMessage struct {
	Identifier string `json:"identifier"`
	Reconnect  bool   `json:"reconnect"`
}

// PingMessage represents a server ping
type PingMessage struct {
	Type    string      `json:"type"`
	Message interface{} `json:"message,omitempty"`
}

func (p *PingMessage) GetType() string {
	return PingType
}

// DisconnectMessage represents a server disconnect message
type DisconnectMessage struct {
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Reconnect bool   `json:"reconnect"`
}

func (d *DisconnectMessage) GetType() string {
	return DisconnectType
}

func NewDisconnectMessage(reason string, reconnect bool) *DisconnectMessage {
	return &DisconnectMessage{Type: "disconnect", Reason: reason, Reconnect: reconnect}
}

// Reply represents an outgoing client message
type Reply struct {
	Type        string      `json:"type,omitempty"`
	Identifier  string      `json:"identifier,omitempty"`
	Message     interface{} `json:"message,omitempty"`
	Reason      string      `json:"reason,omitempty"`
	Reconnect   bool        `json:"reconnect,omitempty"`
	StreamID    string      `json:"stream_id,omitempty"`
	Epoch       string      `json:"epoch,omitempty"`
	Offset      uint64      `json:"offset,omitempty"`
	Sid         string      `json:"sid,omitempty"`
	Restored    bool        `json:"restored,omitempty"`
	RestoredIDs []string    `json:"restored_ids,omitempty"`
}

func (r *Reply) GetType() string {
	return r.Type
}

// PubSubMessageFromJSON takes raw JSON byte array and return the corresponding struct
func PubSubMessageFromJSON(raw []byte) (interface{}, error) {
	smsg := StreamMessage{}

	if err := json.Unmarshal(raw, &smsg); err == nil {
		if smsg.Stream != "" {
			return smsg, nil
		}
	}

	rmsg := RemoteCommandMessage{}

	if err := json.Unmarshal(raw, &rmsg); err != nil {
		return nil, err
	}

	return rmsg, nil
}

// ConfirmationMessage returns a subscription confirmation message for a specified identifier
func ConfirmationMessage(identifier string) string {
	return string(toJSON(Reply{Identifier: identifier, Type: ConfirmedType}))
}

// RejectionMessage returns a subscription rejection message for a specified identifier
func RejectionMessage(identifier string) string {
	return string(toJSON(Reply{Identifier: identifier, Type: RejectedType}))
}

func toJSON(msg Reply) []byte {
	b, err := json.Marshal(&msg)
	if err != nil {
		panic("Failed to build JSON 😲")
	}

	return b
}
