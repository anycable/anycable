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
	statsCollectInterval = 5 * time.Second

	metricsGoroutines      = "goroutines_num"
	metricsClientsNum      = "clients_num"
	metricsUniqClientsNum  = "clients_uniq_num"
	metricsStreamsNum      = "broadcast_streams_num"
	metricsDisconnectQueue = "disconnect_queue_size"

	metricsFailedAuths      = "failed_auths_total"
	metricsReceivedMsg      = "client_msg_total"
	metricsUnknownReceived  = "failed_client_msg_total"
	metricsBroadcastMsg     = "broadcast_msg_total"
	metricsUnknownBroadcast = "failed_broadcast_msg_total"
)

// CommandResult is a result of performing controller action,
// which contains informations about streams to subscribe,
// messages to sent
type CommandResult struct {
	Streams        []string
	StopAllStreams bool
	Transmissions  []string
	Disconnect     bool
	Broadcasts     []*StreamMessage
}

// Controller is an interface describing business-logic handler (e.g. RPC)
type Controller interface {
	Shutdown() error
	Authenticate(sid string, path string, headers *map[string]string) (string, []string, error)
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

// Node represents the whole application
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
func NewNode(config *config.Config, controller Controller, metrics *metrics.Metrics) *Node {
	node := &Node{
		Config:     config,
		Metrics:    metrics,
		controller: controller,
		shutdownCh: make(chan struct{}),
		log:        log.WithFields(log.Fields{"context": "node"}),
	}

	node.hub = NewHub()

	go node.hub.Run()

	node.disconnector = NewDisconnectQueue(node, config.DisconnectRate)

	go node.disconnector.Run()

	node.registerMetrics()
	go node.collectStats()

	return node
}

// HandleCommand parses incoming message from client and
// execute the command (if recognized)
func (n *Node) HandleCommand(s *Session, raw []byte) {
	msg := &Message{}

	n.Metrics.Counter(metricsReceivedMsg).Inc()

	if err := json.Unmarshal(raw, &msg); err != nil {
		n.Metrics.Counter(metricsUnknownReceived).Inc()
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
			n.Metrics.Counter(metricsUnknownReceived).Inc()
			s.Log.Warnf("Unknown command: %s", msg.Command)
		}
	}
}

// HandlePubsub parses incoming pubsub message and broadcast it
func (n *Node) HandlePubsub(raw []byte) {
	msg := &StreamMessage{}

	if err := json.Unmarshal(raw, &msg); err != nil {
		n.Metrics.Counter(metricsUnknownBroadcast).Inc()
		n.log.Warnf("Failed to parse pubsub message '%s' with error: %v", raw, err)
	} else {
		n.Broadcast(msg)
	}
}

// Shutdown stops all services (hub, controller)
func (n *Node) Shutdown() {
	close(n.shutdownCh)

	if n.hub != nil {
		n.hub.Shutdown()

		active := len(n.hub.sessions)

		if active > 0 {
			n.log.Infof("Closing active connections: %d", active)
			// Close all registered sessions
			for _, session := range n.hub.sessions {
				session.Disconnect("Shutdown", CloseGoingAway)
			}

			n.log.Info("All active connections closed")

			// Wait to make sure that disconnect queue is not empty
			time.Sleep(time.Second)
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
	id, transmissions, err := n.controller.Authenticate(s.UID, path, headers)

	transmit(s, transmissions)

	if err == nil {
		s.Identifiers = id
		s.connected = true

		n.hub.register <- s
	} else {
		n.Metrics.Counter(metricsFailedAuths).Inc()
	}

	return err
}

// Subscribe subscribes session to a channel
func (n *Node) Subscribe(s *Session, msg *Message) {
	s.mu.Lock()

	if _, ok := s.subscriptions[msg.Identifier]; ok {
		s.Log.Warnf("Already subscribed to %s", msg.Identifier)
		s.mu.Unlock()
		return
	}

	res, err := n.controller.Subscribe(s.UID, s.Identifiers, msg.Identifier)

	if err != nil {
		s.Log.Errorf("Subscribe error: %v", err)
	} else {
		s.subscriptions[msg.Identifier] = true
		s.Log.Debugf("Subscribed to channel: %s", msg.Identifier)
	}

	s.mu.Unlock()

	if res != nil {
		n.handleCommandReply(s, msg, res)
	}
}

// Unsubscribe unsubscribes session from a channel
func (n *Node) Unsubscribe(s *Session, msg *Message) {
	s.mu.Lock()

	if _, ok := s.subscriptions[msg.Identifier]; !ok {
		s.Log.Warnf("Unknown subscription %s", msg.Identifier)
		s.mu.Unlock()
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

	s.mu.Unlock()

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
	n.Metrics.Counter(metricsBroadcastMsg).Inc()
	n.log.Debugf("Incoming pubsub message: %v", msg)
	n.hub.broadcast <- msg
}

// Disconnect adds session to disconnector queue and unregister session from hub
func (n *Node) Disconnect(s *Session) {
	n.hub.unregister <- s
	n.disconnector.Enqueue(s)
}

// DisconnectNow execute disconnect on controller
func (n *Node) DisconnectNow(s *Session) error {
	sessionSubscriptions := subscriptionsList(s.subscriptions)

	s.Log.Debugf("Disconnect %s %s %v %v", s.Identifiers, s.path, s.headers, sessionSubscriptions)

	err := n.controller.Disconnect(
		s.UID,
		s.Identifiers,
		sessionSubscriptions,
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
		defer s.Disconnect("Command Failed", CloseAbnormalClosure)
	}

	if reply.StopAllStreams {
		n.hub.unsubscribe <- &SubscriptionInfo{session: s.UID, identifier: msg.Identifier}
	}

	if reply.Streams != nil {
		for _, stream := range reply.Streams {
			n.hub.subscribe <- &SubscriptionInfo{session: s.UID, stream: stream, identifier: msg.Identifier}
		}
	}

	if reply.Broadcasts != nil {
		for _, broadcast := range reply.Broadcasts {
			n.Broadcast(broadcast)
		}
	}

	if reply.Transmissions != nil {
		transmit(s, reply.Transmissions)
	}
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
	n.Metrics.Gauge(metricsGoroutines).Set(runtime.NumGoroutine())

	n.Metrics.Gauge(metricsClientsNum).Set(n.hub.Size())
	n.Metrics.Gauge(metricsUniqClientsNum).Set(n.hub.UniqSize())
	n.Metrics.Gauge(metricsStreamsNum).Set(n.hub.StreamsSize())
	n.Metrics.Gauge(metricsDisconnectQueue).Set(n.disconnector.Size())
}

func (n *Node) registerMetrics() {
	n.Metrics.RegisterGauge(metricsGoroutines, "The number of Go routines")

	n.Metrics.RegisterGauge(metricsClientsNum, "The number of active clients")
	n.Metrics.RegisterGauge(metricsUniqClientsNum, "The number of unique clients (with respect to connection identifiers)")
	n.Metrics.RegisterGauge(metricsStreamsNum, "The number of active broadcasting streams")
	n.Metrics.RegisterGauge(metricsDisconnectQueue, "The size of delayed disconnect")

	n.Metrics.RegisterCounter(metricsFailedAuths, "The total number of failed authentication attempts")
	n.Metrics.RegisterCounter(metricsReceivedMsg, "The total number of received messages from clients")
	n.Metrics.RegisterCounter(metricsUnknownReceived, "The total number of unrecognized messages received from clients")
	n.Metrics.RegisterCounter(metricsBroadcastMsg, "The total number of messages received through PubSub (for broadcast)")
	n.Metrics.RegisterCounter(metricsUnknownBroadcast, "The total number of unrecognized messages received through PubSub")
}

func subscriptionsList(m map[string]bool) []string {
	keys := make([]string, len(m))
	i := 0

	for k := range m {
		keys[i] = k
		i++
	}
	return keys
}
