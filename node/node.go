package node

import (
	"encoding/json"

	"github.com/anycable/anycable-go/config"
	"github.com/apex/log"
)

const (
	// PING stores the "ping" message identifier
	PING = "ping"
)

// CommandResult is a result of performing controller action,
// which contains informations about streams to subscribe,
// messages to sent
type CommandResult struct {
	Streams       []string
	StopStreams   bool
	Transmissions []string
	Disconnect    bool
}

// Controller is an interface describing business-logic handler (e.g. RPC)
type Controller interface {
	Shutdown() error
	Authenticate(path string, headers *map[string]string) (string, error)
	Subscribe(sid string, id string, channel string) (*CommandResult, error)
	Unsubscribe(sid string, id string, channel string) (*CommandResult, error)
	Perform(sid string, id string, channel string, data string) (*CommandResult, error)
}

// Message represents incoming client message
type Message struct {
	Command    string `json:"command"`
	Identifier string `json:"identifier"`
	Data       string `json:"data"`
}

// PubSubMessage represents data passing through pubsub channel
type PubSubMessage struct {
	Stream string `json:"stream"`
	Data   string `json:"data"`
}

// Node represents the whole applicaton
type Node struct {
	hub        *Hub
	controller Controller
	Config     *config.Config
}

// NewNode builds new node struct
func NewNode(config *config.Config, controller Controller) *Node {
	hub := NewHub()

	go hub.Run()

	return &Node{
		Config:     config,
		hub:        hub,
		controller: controller,
	}
}

// HandleCommand parses incoming message from client and
// execute the command (if recognized)
func (n *Node) HandleCommand(s *Session, raw []byte) {
	msg := &Message{}

	if err := json.Unmarshal(raw, &msg); err != nil {
		s.Log.Warnf("Failed to parse incoming message '%s' with error: %v", raw, err)
	} else {
		s.Log.Debugf("Incoming message: %s", msg)
		switch msg.Command {
		case "subscribe":
			n.Subscribe(s, msg)
		case "unsubscribe":
			n.Unsubscribe(s, msg)
		case "message":
			n.Perform(s, msg)
		default:
			s.Log.Warnf("Unknown command: %s", msg.Command)
		}
	}
}

// HandlePubsub parses incoming pubsub message and broadcast it
func (n *Node) HandlePubsub(raw []byte) {
	msg := &PubSubMessage{}
	if err := json.Unmarshal(raw, &msg); err != nil {
		log.Warnf("Failed to parse pubsub message '%s' with error: %v", raw, err)
	} else {
		log.Debugf("Incoming pubsub message: %v", msg)
		// n.Broadcast(msg.Stream, msg.Data)
	}
}

// Shutdown stops all services (hub, controller)
func (n *Node) Shutdown() {
	if n.hub != nil {
		n.hub.Shutdown()
	}

	if n.controller != nil {
		n.controller.Shutdown()
	}
}

// Authenticate calls controller to perform authentication and return connection identifiers
func (n *Node) Authenticate(path string, headers *map[string]string) (string, error) {
	return n.controller.Authenticate(path, headers)
}

// Subscribe subscribes session to a channel
func (n *Node) Subscribe(s *Session, msg *Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.subscriptions[msg.Identifier]; ok {
		s.Log.Warnf("Already subscribed to %s", msg.Identifier)
		return
	}

	res, err := n.controller.Subscribe(s.UID, s.Identifiers, msg.Identifier)

	if err != nil {
		s.Log.Errorf("Subscribe error: %v", err)
		return
	}

	s.subscriptions[msg.Identifier] = true

	s.Log.Debugf("Subscribed to channel: %s", msg.Identifier)

	n.handleCommandReply(s, msg, res)
}

// Unsubscribe unsubscribes session from a channel
func (n *Node) Unsubscribe(s *Session, msg *Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.subscriptions[msg.Identifier]; !ok {
		s.Log.Warnf("Unknown subscription %s", msg.Identifier)
		return
	}

	res, err := n.controller.Unsubscribe(s.UID, s.Identifiers, msg.Identifier)

	if err != nil {
		s.Log.Errorf("Unsubscribe error: %v", err)
		return
	}

	delete(s.subscriptions, msg.Identifier)

	s.Log.Debugf("Unsubscribed from channel: %s", msg.Identifier)

	n.handleCommandReply(s, msg, res)
}

// Perform executes client command
func (n *Node) Perform(s *Session, msg *Message) {
	s.mu.Lock()

	if _, ok := s.subscriptions[msg.Identifier]; !ok {
		s.Log.Warnf("Unknown subscription %s", msg.Identifier)
		s.mu.Unlock()
		return
	}

	s.mu.Unlock()

	res, err := n.controller.Perform(s.UID, s.Identifiers, msg.Identifier, msg.Data)

	if err != nil {
		s.Log.Errorf("Perform error: %v", err)
		return
	}

	s.Log.Debugf("Perform result: %v", res)

	n.handleCommandReply(s, msg, res)
}

func transmit(s *Session, transmissions []string) {
	for _, msg := range transmissions {
		s.Send([]byte(msg))
	}
}

func (n *Node) handleCommandReply(s *Session, msg *Message, reply *CommandResult) {
	if reply.Disconnect {
		defer s.Disconnect("Command Failed")
	}

	if reply.StopStreams {
		n.hub.unsubscribe <- &SubscriptionInfo{session: s.UID, identifier: msg.Identifier}
	}

	for _, stream := range reply.Streams {
		n.hub.subscribe <- &SubscriptionInfo{session: s.UID, stream: stream, identifier: msg.Identifier}
	}

	transmit(s, reply.Transmissions)
}
