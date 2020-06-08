// Package common contains struts and interfaces shared between multiple components
package common

import (
	"encoding/json"
	"fmt"
)

// SessionEnv represents the underlying HTTP connection data:
// URL and request headers
type SessionEnv struct {
	URL             string
	Headers         *map[string]string
	ConnectionState *map[string]string
}

// NewSessionEnv builds a new SessionEnv
func NewSessionEnv(url string, headers *map[string]string) *SessionEnv {
	state := make(map[string]string)
	return &SessionEnv{
		URL:             url,
		Headers:         headers,
		ConnectionState: &state,
	}
}

// MergeConnectionState is update the current ConnectionState from the given map.
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

// CallResult contains shared RPC result fields
type CallResult struct {
	Transmissions []string
	Broadcasts    []*StreamMessage
	CState        map[string]string
}

// ConnectResult is a result of initializing a connection (calling a Connect method)
type ConnectResult struct {
	Identifier    string
	Transmissions []string
	Broadcasts    []*StreamMessage
	CState        map[string]string
}

// ToCallResult returns the corresponding CallResult
func (c *ConnectResult) ToCallResult() *CallResult {
	res := CallResult{Transmissions: c.Transmissions, Broadcasts: c.Broadcasts}
	if c.CState != nil {
		res.CState = c.CState
	}
	return &res
}

// CommandResult is a result of performing controller action,
// which contains informations about streams to subscribe,
// messages to sent and broadcast.
// It's a communication "protocol" between a node and a controller.
type CommandResult struct {
	StopAllStreams bool
	Disconnect     bool
	Streams        []string
	StoppedStreams []string
	Transmissions  []string
	Broadcasts     []*StreamMessage
	CState         map[string]string
}

// ToCallResult returns the corresponding CallResult
func (c *CommandResult) ToCallResult() *CallResult {
	res := CallResult{Transmissions: c.Transmissions, Broadcasts: c.Broadcasts}
	if c.CState != nil {
		res.CState = c.CState
	}
	return &res
}

// Message represents incoming client message
type Message struct {
	Command    string `json:"command"`
	Identifier string `json:"identifier"`
	Data       string `json:"data"`
}

// StreamMessage represents a pub/sub message to be sent to stream
type StreamMessage struct {
	Stream string `json:"stream"`
	Data   string `json:"data"`
}

// RemoteCommandMessage represents a pub/sub message with a remote command (e.g., disconnect)
type RemoteCommandMessage struct {
	Command string          `json:"command,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// RemoteDisconnectMessage contains information required to disconnect a session
type RemoteDisconnectMessage struct {
	Identifier string `json:"identifier"`
	Reconnect  bool   `json:"reconnect"`
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

	if rmsg.Command == "disconnect" {
		dmsg := RemoteDisconnectMessage{}

		if err := json.Unmarshal(rmsg.Payload, &dmsg); err != nil {
			return nil, err
		}

		return dmsg, nil
	}

	return nil, fmt.Errorf("Unknown message: %s", raw)
}
