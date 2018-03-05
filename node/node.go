package node

import (
	"encoding/json"
	"runtime"
	"time"

	"github.com/anycable/anycable-go/config"
	"github.com/anycable/anycable-go/metrics"
	"github.com/apex/log"
)

const (
	// PING stores the "ping" message identifier
	PING = "ping"

	// How often update node stats
	statsCollectInterval = 2 * time.Second
)

// CommandResult is a result of performing controller action,
// which contains informations about streams to subscribe,
// messages to sent
type CommandResult struct {
	Streams        []string
	StopAllStreams bool
	Transmissions  []string
	Disconnect     bool
}

// Controller is an interface describing business-logic handler (e.g. RPC)
type Controller interface {
	Shutdown() error
	Authenticate(path string, headers *map[string]string) (string, []string, error)
	Subscribe(sid string, id string, channel string) (*CommandResult, error)
	Unsubscribe(sid string, id string, channel string) (*CommandResult, error)
	Perform(sid string, id string, channel string, data string) (*CommandResult, error)
	Disconnect(sid string, id string, subscriptions []string, path string, headers *map[string]string) error
}

// Message represents incoming client message
type Message struct {
	Command    string `json:"command"`
	Identifier string `json:"identifier"`
	Data       string `json:"data"`
}

// StreamMessage represents a message to be sent to stream
type StreamMessage struct {
	Stream string `json:"stream"`
	Data   string `json:"data"`
}

// Node represents the whole applicaton
type Node struct {
	Config  *config.Config
	Metrics *metrics.Metrics

	hub          *Hub
	controller   Controller
	disconnector *DisconnectQueue
	shutdownCh   chan struct{}
	log          *log.Entry
}

// NewNode builds new node struct
func NewNode(config *config.Config, controller Controller) *Node {
	node := &Node{
		Config:     config,
		Metrics:    metrics.NewMetrics(config),
		controller: controller,
		shutdownCh: make(chan struct{}),
		log:        log.WithFields(log.Fields{"context": "node"}),
	}

	node.hub = NewHub()

	go node.hub.Run()

	node.disconnector = NewDisconnectQueue(node, config.DisconnectRate)

	go node.disconnector.Run()

	node.registerMetrics()

	go node.Metrics.Run()
	go node.collectStats()

	return node
}

// HandleCommand parses incoming message from client and
// execute the command (if recognized)
func (n *Node) HandleCommand(s *Session, raw []byte) {
	msg := &Message{}

	n.Metrics.Counter("client_msg_count").Inc()

	if err := json.Unmarshal(raw, &msg); err != nil {
		n.Metrics.Counter("unknown_client_msg_count").Inc()
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
			n.Metrics.Counter("unknown_client_msg_count").Inc()
			s.Log.Warnf("Unknown command: %s", msg.Command)
		}
	}
}

// HandlePubsub parses incoming pubsub message and broadcast it
func (n *Node) HandlePubsub(raw []byte) {
	msg := &StreamMessage{}

	n.Metrics.Counter("broadcast_msg_count").Inc()

	if err := json.Unmarshal(raw, &msg); err != nil {
		n.Metrics.Counter("unknown_broadcast_msg_count").Inc()
		n.log.Warnf("Failed to parse pubsub message '%s' with error: %v", raw, err)
	} else {
		n.log.Debugf("Incoming pubsub message: %v", msg)
		n.Broadcast(msg)
	}
}

// Shutdown stops all services (hub, controller)
func (n *Node) Shutdown() {
	n.log.Infof("Shutting down...")

	close(n.shutdownCh)

	n.Metrics.Shutdown()

	if n.hub != nil {
		n.hub.Shutdown()

		active := len(n.hub.sessions)

		if active > 0 {
			n.log.Infof("Closing active connections: %d", active)
			// Close all registered sessions
			for _, session := range n.hub.sessions {
				session.Disconnect("Shutdown")
			}
		}
	}

	if n.disconnector != nil {
		err := n.disconnector.Shutdown()

		if err != nil {
			n.log.Warnf("%v", err)
		}
	}

	if n.controller != nil {
		err := n.controller.Shutdown()

		if err != nil {
			n.log.Warnf("%v", err)
		}
	}
}

// Authenticate calls controller to perform authentication.
// If authentication is successful, session is registered with a hub.
func (n *Node) Authenticate(s *Session, path string, headers *map[string]string) error {
	id, transmissions, err := n.controller.Authenticate(path, headers)

	if err == nil {
		s.Identifiers = id
		s.connected = true

		transmit(s, transmissions)

		n.hub.register <- s
	} else {
		n.Metrics.Counter("auth_failures_count").Inc()
	}

	return err
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
	} else {
		s.subscriptions[msg.Identifier] = true
		s.Log.Debugf("Subscribed to channel: %s", msg.Identifier)
	}

	if res != nil {
		n.handleCommandReply(s, msg, res)
	}
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
	} else {
		// Make sure to remove all streams subscriptions
		res.StopAllStreams = true

		delete(s.subscriptions, msg.Identifier)

		s.Log.Debugf("Unsubscribed from channel: %s", msg.Identifier)
	}

	if res != nil {
		n.handleCommandReply(s, msg, res)
	}
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
	} else {
		s.Log.Debugf("Perform result: %v", res)
	}

	if res != nil {
		n.handleCommandReply(s, msg, res)
	}
}

// Broadcast message to stream
func (n *Node) Broadcast(msg *StreamMessage) {
	n.hub.broadcast <- msg
}

// Disconnect adds session to disconnector queue and unregister session from hub
func (n *Node) Disconnect(s *Session) {
	n.hub.unregister <- s
	n.disconnector.Enqueue(s)
}

// DisconnectNow execute disconnect on controller
func (n *Node) DisconnectNow(s *Session) error {
	s.Log.Debugf("Disconnect %s %s %v %v", s.Identifiers, s.path, s.headers, s.subscriptions)

	err := n.controller.Disconnect(
		s.UID,
		s.Identifiers,
		subscriptionsList(s.subscriptions),
		s.path,
		&s.headers,
	)

	if err != nil {
		s.Log.Errorf("Disconnect error: %v", err)
	}

	return err
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

	if reply.StopAllStreams {
		n.hub.unsubscribe <- &SubscriptionInfo{session: s.UID, identifier: msg.Identifier}
	}

	for _, stream := range reply.Streams {
		n.hub.subscribe <- &SubscriptionInfo{session: s.UID, stream: stream, identifier: msg.Identifier}
	}

	transmit(s, reply.Transmissions)
}

func (n *Node) collectStats() {
	for {
		select {
		case <-n.shutdownCh:
			return
		case <-time.After(statsCollectInterval):
			n.collectStatsOnce()
		}
	}
}

func (n *Node) collectStatsOnce() {
	n.Metrics.Gauge("goroutines_num").Set(runtime.NumGoroutine())

	n.Metrics.Gauge("clients_num").Set(n.hub.Size())
	n.Metrics.Gauge("uniq_clients_num").Set(n.hub.UniqSize())
	n.Metrics.Gauge("streams_num").Set(n.hub.StreamsSize())
	n.Metrics.Gauge("disconnect_queue_size").Set(n.disconnector.Size())
}

func (n *Node) registerMetrics() {
	n.Metrics.RegisterGauge("goroutines_num")

	n.Metrics.RegisterGauge("clients_num")
	n.Metrics.RegisterGauge("uniq_clients_num")
	n.Metrics.RegisterGauge("streams_num")
	n.Metrics.RegisterGauge("disconnect_queue_size")

	n.Metrics.RegisterCounter("auth_failures_count")
	n.Metrics.RegisterCounter("client_msg_count")
	n.Metrics.RegisterCounter("unknown_client_msg_count")
	n.Metrics.RegisterCounter("broadcast_msg_count")
	n.Metrics.RegisterCounter("unknown_broadcast_msg_count")
}

func subscriptionsList(m map[string]bool) []string {
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
