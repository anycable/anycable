// Package common contains struts and interfaces shared between multiple components
package common

import (
	"encoding/json"
	"log/slog"

	"github.com/anycable/anycable-go/logger"
	"github.com/anycable/anycable-go/utils"
)

// Command result status
const (
	SUCCESS = iota
	FAILURE
	ERROR
)

func StatusName(status int) string {
	switch status {
	case SUCCESS:
		return "success"
	case FAILURE:
		return "failure"
	case ERROR:
		return "error"
	default:
		return "unknown"
	}
}

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

	WhisperType = "whisper"
)

// Disconnect reasons
const (
	SERVER_RESTART_REASON    = "server_restart"
	REMOTE_DISCONNECT_REASON = "remote"
	IDLE_TIMEOUT_REASON      = "idle_timeout"
	NO_PONG_REASON           = "no_pong"
	UNAUTHORIZED_REASON      = "unauthorized"
)

// Reserver state fields
const (
	WHISPER_STREAM_STATE = "$w"
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

func (st *SessionEnv) RemoveChannelState(id string) {
	delete((*st.ChannelStates), id)
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

func (c *ConnectResult) LogValue() slog.Value {
	if c == nil {
		return slog.StringValue("nil")
	}

	return slog.GroupValue(
		slog.String("status", StatusName(c.Status)),
		slog.Any("transmissions", logger.CompactValues(c.Transmissions)),
		slog.Any("broadcasts", c.Broadcasts),
		slog.String("identifier", c.Identifier),
		slog.Int("disconnect_interest", c.DisconnectInterest),
		slog.Any("cstate", c.CState),
		slog.Any("istate", c.IState),
	)
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

func (c *CommandResult) LogValue() slog.Value {
	if c == nil {
		return slog.StringValue("nil")
	}

	return slog.GroupValue(
		slog.String("status", StatusName(c.Status)),
		slog.Any("streams", logger.CompactValues(c.Streams)),
		slog.Any("transmissions", logger.CompactValues(c.Transmissions)),
		slog.Any("stopped_streams", logger.CompactValues(c.StoppedStreams)),
		slog.Bool("stop_all_streams", c.StopAllStreams),
		slog.Any("broadcasts", c.Broadcasts),
		slog.Bool("disconnect", c.Disconnect),
		slog.Int("disconnect_interest", c.DisconnectInterest),
		slog.Any("cstate", c.CState),
		slog.Any("istate", c.IState),
	)
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

func (hp *HistoryPosition) LogValue() slog.Value {
	if hp == nil {
		return slog.StringValue("nil")
	}

	return slog.GroupValue(slog.String("epoch", hp.Epoch), slog.Uint64("offset", hp.Offset))
}

// HistoryRequest represents a client's streams state (offsets) or a timestamp since
// which we should return the messages for the current streams
type HistoryRequest struct {
	// Since is UTC timestamp in ms
	Since int64 `json:"since,omitempty"`
	// Streams contains the information of last offsets/epoch received for a particular stream
	Streams map[string]HistoryPosition `json:"streams,omitempty"`
}

func (hr *HistoryRequest) LogValue() slog.Value {
	if hr == nil {
		return slog.StringValue("nil")
	}

	return slog.GroupValue(slog.Int64("since", hr.Since), slog.Any("streams", hr.Streams))
}

// Message represents incoming client message
type Message struct {
	Command    string         `json:"command"`
	Identifier string         `json:"identifier"`
	Data       interface{}    `json:"data,omitempty"`
	History    HistoryRequest `json:"history,omitempty"`
}

func (m *Message) LogValue() slog.Value {
	if m == nil {
		return slog.StringValue("nil")
	}

	return slog.GroupValue(
		slog.String("command", m.Command),
		slog.String("identifier", m.Identifier),
		slog.Any("data", logger.CompactAny(m.Data)),
		slog.Any("history", m.History),
	)
}

// StreamMessageMetadata describes additional information about a stream message
// which can be used to modify delivery behavior
type StreamMessageMetadata struct {
	ExcludeSocket string `json:"exclude_socket,omitempty"`
	// BroadcastType defines the message type to be used for messages sent to clients
	BroadcastType string `json:"broadcast_type,omitempty"`
	// Transient defines whether this message should be stored in the history
	Transient bool `json:"transient,omitempty"`
}

func (smm *StreamMessageMetadata) LogValue() slog.Value {
	if smm == nil {
		return slog.StringValue("nil")
	}

	return slog.GroupValue(slog.String("exclude_socket", smm.ExcludeSocket))
}

// StreamMessage represents a pub/sub message to be sent to stream
type StreamMessage struct {
	Stream string                 `json:"stream"`
	Data   string                 `json:"data"`
	Meta   *StreamMessageMetadata `json:"meta,omitempty"`

	// Offset is the position of this message in the stream
	Offset uint64
	// Epoch is the uniq ID of the current storage state
	Epoch string
}

func (sm *StreamMessage) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("stream", sm.Stream),
		slog.Any("data", logger.CompactValue(sm.Data)),
	}

	if sm.Epoch != "" {
		attrs = append(attrs, slog.Uint64("offset", sm.Offset), slog.String("epoch", sm.Epoch))
	}

	if sm.Meta != nil {
		attrs = append(attrs, slog.Any("meta", sm.Meta))
	}

	return slog.GroupValue(attrs...)
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

func (m *RemoteCommandMessage) LogValue() slog.Value {
	if m == nil {
		return slog.StringValue("nil")
	}

	return slog.GroupValue(slog.String("command", m.Command), slog.Any("payload", m.Payload))
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

func (m *RemoteDisconnectMessage) LogValue() slog.Value {
	if m == nil {
		return slog.StringValue("nil")
	}

	return slog.GroupValue(slog.String("ids", m.Identifier), slog.Bool("reconnect", m.Reconnect))
}

// PingMessage represents a server ping
type PingMessage struct {
	Type    string      `json:"type"`
	Message interface{} `json:"message,omitempty"`
}

func (p *PingMessage) LogValue() slog.Value {
	return slog.GroupValue(slog.String("type", p.Type), slog.Any("message", p.Message))
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

func (d *DisconnectMessage) LogValue() slog.Value {
	return slog.GroupValue(slog.String("type", d.Type), slog.String("reason", d.Reason), slog.Bool("reconnect", d.Reconnect))
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

func (r *Reply) LogValue() slog.Value {
	if r == nil {
		return slog.StringValue("nil")
	}

	attrs := []slog.Attr{}

	if r.Type != "" {
		attrs = append(attrs, slog.String("type", r.Type))
	}

	if r.Identifier != "" {
		attrs = append(attrs, slog.String("identifier", r.Identifier))
	}

	if r.Message != nil {
		attrs = append(attrs, slog.Any("message", logger.CompactAny(r.Message)))
	}

	if r.Reason != "" {
		attrs = append(attrs, slog.String("reason", r.Reason), slog.Bool("reconnect", r.Reconnect))
	}

	if r.StreamID != "" {
		attrs = append(attrs, slog.String("stream_id", r.StreamID), slog.String("epoch", r.Epoch), slog.Uint64("offset", r.Offset))
	}

	if r.Sid != "" {
		attrs = append(attrs, slog.String("sid", r.Sid), slog.Bool("restored", r.Restored), slog.Any("restored_ids", r.RestoredIDs))
	}

	return slog.GroupValue(attrs...)
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

	batch := []*StreamMessage{}

	if err := json.Unmarshal(raw, &batch); err == nil {
		if len(batch) > 0 && batch[0].Stream != "" {
			return batch, nil
		}
	}

	rmsg := RemoteCommandMessage{}

	if err := json.Unmarshal(raw, &rmsg); err != nil {
		return nil, err
	}

	return rmsg, nil
}

// WelcomeMessage for a session ID
func WelcomeMessage(sid string) string {
	return string(utils.ToJSON(Reply{Sid: sid, Type: WelcomeType}))
}

// ConfirmationMessage returns a subscription confirmation message for a specified identifier
func ConfirmationMessage(identifier string) string {
	return string(utils.ToJSON(Reply{Identifier: identifier, Type: ConfirmedType}))
}

// RejectionMessage returns a subscription rejection message for a specified identifier
func RejectionMessage(identifier string) string {
	return string(utils.ToJSON(Reply{Identifier: identifier, Type: RejectedType}))
}

// DisconnectionMessage returns a disconnect message with the specified reason and reconnect flag
func DisconnectionMessage(reason string, reconnect bool) string {
	return string(utils.ToJSON(DisconnectMessage{Type: DisconnectType, Reason: reason, Reconnect: reconnect}))
}
