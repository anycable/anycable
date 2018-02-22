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
	hub    *Hub
	Config *config.Config
}

// NewNode builds new node struct
func NewNode(config *config.Config) *Node {
	hub := NewHub()

	go hub.Run()

	return &Node{
		Config: config,
		hub:    hub,
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
			n.Subscribe(s.Log, s, msg)
		case "unsubscribe":
			n.Unsubscribe(s.Log, s, msg)
		case "message":
			n.Perform(s.Log, s, msg)
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

// Authenticate calls RPC server to perform authentication and return connection identifiers
func (n *Node) Authenticate(path string, headers map[string]string) (string, error) {
	return "", nil
}

// Subscribe subscribes session to a channel
func (n *Node) Subscribe(log *log.Entry, s *Session, msg *Message) {
	if _, ok := s.subscriptions[msg.Identifier]; ok {
		log.Warnf("Already subscribed to %s", msg.Identifier)
		return
	}

	// res, err := n.rpc.Subscribe(s.identifiers, msg.Identifier)

	// if err != nil {
	// 	log.Errorf("RPC Subscribe Error: %v", err)
	// 	// TODO: need a way to tell client to retry later
	// 	return
	// }

	// if res.Status.String() == "SUCCESS" {
	// 	s.subscriptions[msg.Identifier] = true
	// }

	// log.Debugf("Subscribe %s", res)

	// HandleReply(s, msg, res)
}

// Unsubscribe unsubscribes session from a channel
func (n *Node) Unsubscribe(log *log.Entry, s *Session, msg *Message) {
	if _, ok := s.subscriptions[msg.Identifier]; !ok {
		log.Warnf("Unknown subscription %s", msg.Identifier)
		return
	}

	// res, err := rpc.Unsubscribe(s.identifiers, msg.Identifier)

	// if err != nil {
	// 	log.Errorf("RPC Unsubscribe Error: %v", err)
	// 	// TODO: need a way to tell client to retry later
	// 	return
	// }

	// if res.Status.String() == "SUCCESS" {
	// 	delete(s.subscriptions, msg.Identifier)
	// }

	// HandleReply(s, msg, res)
}

// Perform executes client command
func (n *Node) Perform(log *log.Entry, s *Session, msg *Message) {
	if _, ok := s.subscriptions[msg.Identifier]; !ok {
		log.Warnf("Unknown subscription %s", msg.Identifier)
		return
	}

	// res, err := rpc.Perform(s.identifiers, msg.Identifier, msg.Data)

	// if err != nil {
	// 	log.Errorf("RPC Perform Error: %v", err)
	// 	// TODO: need a way to tell client to retry later
	// 	return
	// }

	// log.Debugf("Perform %s", res)

	// HandleReply(s, msg, res)
}

// func (n *Node) Dissected(s *Session) {
// 	app.Pinger.Decrement()

// 	hub.unregister <- s

// 	app.Dissector.Notify(s)
// }

// func (n *Node) BroadcastAll(message []byte) {
// 	hub.broadcast <- message
// }

// func Transmit(s *Session, transmissions []string) {
// 	for _, msg := range transmissions {
// 		s.send <- []byte(msg)
// 	}
// }

// func HandleReply(s *Session, msg *Message, reply *pb.CommandResponse) {
// 	if reply.Status.String() == "ERROR" {
// 		log.Errorf("Application error: %s", reply.ErrorMsg)
// 	}

// 	if reply.Dissect {
// 		defer CloseWS(s.ws, "Command Failed")
// 	}

// 	if reply.StopStreams {
// 		hub.unsubscribe <- &SubscriptionInfo{s: s, identifier: msg.Identifier}
// 	}

// 	for _, s := range reply.Streams {
// 		hub.subscribe <- &SubscriptionInfo{s: s, stream: s, identifier: msg.Identifier}
// 	}

// 	Transmit(s, reply.Transmissions)
// }
