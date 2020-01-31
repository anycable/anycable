// Package common contains struts and interfaces shared between multiple components
package common

// SessionEnv represents the underlying HTTP connection data:
// URL and request headers
type SessionEnv struct {
	URL     string
	Headers *map[string]string
}

// CallResult contains shared RPC result fields
type CallResult struct {
	Transmissions []string
	Broadcasts    []*StreamMessage
}

// ConnectResult is a result of initializing a connection (calling a Connect method)
type ConnectResult struct {
	Identifier    string
	Transmissions []string
	Broadcasts    []*StreamMessage
}

// ToCallResult returns the corresponding CallResult
func (c *ConnectResult) ToCallResult() *CallResult {
	return &CallResult{c.Transmissions, c.Broadcasts}
}

// CommandResult is a result of performing controller action,
// which contains informations about streams to subscribe,
// messages to sent and broadcast.
// It's a communication "protocol" between a node and a controller.
type CommandResult struct {
	StopAllStreams bool
	Disconnect     bool
	Streams        []string
	Transmissions  []string
	Broadcasts     []*StreamMessage
}

// ToCallResult returns the corresponding CallResult
func (c *CommandResult) ToCallResult() *CallResult {
	return &CallResult{c.Transmissions, c.Broadcasts}
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
