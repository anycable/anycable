// Package common contains struts and interfaces shared between multiple components
package common

// SessionEnv represents the underlying HTTP connection data:
// URL path and request headers
type SessionEnv struct {
	Path    string
	Headers *map[string]string
}

// CommandResult is a result of performing controller action,
// which contains informations about streams to subscribe,
// messages to sent and broadcast.
// It's a communication "protocol" between a node and a controller.
type CommandResult struct {
	Streams        []string
	StopAllStreams bool
	Transmissions  []string
	Disconnect     bool
	Broadcasts     []*StreamMessage
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
