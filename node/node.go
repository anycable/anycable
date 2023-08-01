package node

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/hub"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/utils"
	"github.com/anycable/anycable-go/ws"
	"github.com/apex/log"
)

const (
	metricsGoroutines      = "goroutines_num"
	metricsMemSys          = "mem_sys_bytes"
	metricsClientsNum      = "clients_num"
	metricsUniqClientsNum  = "clients_uniq_num"
	metricsStreamsNum      = "broadcast_streams_num"
	metricsDisconnectQueue = "disconnect_queue_size"

	metricsFailedAuths           = "failed_auths_total"
	metricsReceivedMsg           = "client_msg_total"
	metricsFailedCommandReceived = "failed_client_msg_total"
	metricsBroadcastMsg          = "broadcast_msg_total"
	metricsUnknownBroadcast      = "failed_broadcast_msg_total"

	metricsSentMsg    = "server_msg_total"
	metricsFailedSent = "failed_server_msg_total"

	metricsDataSent     = "data_sent_total"
	metricsDataReceived = "data_rcvd_total"
)

// AppNode describes a basic node interface
//
//go:generate mockery --name AppNode --output "../node_mocks" --outpkg node_mocks
type AppNode interface {
	HandlePubSub(msg []byte)
	LookupSession(id string) *Session
	Authenticate(s *Session, opts ...AuthOption) (*common.ConnectResult, error)
	Authenticated(s *Session, identifiers string)
	Subscribe(s *Session, msg *common.Message) (*common.CommandResult, error)
	Unsubscribe(s *Session, msg *common.Message) (*common.CommandResult, error)
	Perform(s *Session, msg *common.Message) (*common.CommandResult, error)
	Disconnect(s *Session) error
}

// Connection represents underlying connection
type Connection interface {
	Write(msg []byte, deadline time.Time) error
	WriteBinary(msg []byte, deadline time.Time) error
	Read() ([]byte, error)
	Close(code int, reason string)
}

// Node represents the whole application
type Node struct {
	metrics metrics.Instrumenter

	config       *Config
	hub          *hub.Hub
	broker       broker.Broker
	controller   Controller
	disconnector Disconnector
	shutdownCh   chan struct{}
	shutdownMu   sync.Mutex
	closed       bool
	log          *log.Entry
}

var _ AppNode = (*Node)(nil)

// NewNode builds new node struct
func NewNode(controller Controller, metrics *metrics.Metrics, config *Config) *Node {
	node := &Node{
		metrics:    metrics,
		config:     config,
		controller: controller,
		shutdownCh: make(chan struct{}),
		log:        log.WithFields(log.Fields{"context": "node"}),
	}

	node.hub = hub.NewHub(config.HubGopoolSize)

	if metrics != nil {
		node.registerMetrics()
	}

	return node
}

// Start runs all the required goroutines
func (n *Node) Start() error {
	go n.hub.Run()
	go n.collectStats()

	if err := n.broker.Start(); err != nil {
		return err
	}

	return nil
}

// SetDisconnector set disconnector for the node
func (n *Node) SetDisconnector(d Disconnector) {
	n.disconnector = d
}

func (n *Node) SetBroker(b broker.Broker) {
	n.broker = b
}

// Return current instrumenter for the node
func (n *Node) Instrumenter() metrics.Instrumenter {
	return n.metrics
}

// HandleCommand parses incoming message from client and
// execute the command (if recognized)
func (n *Node) HandleCommand(s *Session, msg *common.Message) (err error) {
	s.Log.Debugf("Incoming message: %v", msg)
	switch msg.Command {
	case "subscribe":
		_, err = n.Subscribe(s, msg)
	case "unsubscribe":
		_, err = n.Unsubscribe(s, msg)
	case "message":
		_, err = n.Perform(s, msg)
	case "history":
		err = n.History(s, msg)
	default:
		err = fmt.Errorf("Unknown command: %s", msg.Command)
	}

	return
}

// HandleBroadcast parses incoming broadcast message, record it and re-transmit to other nodes
func (n *Node) HandleBroadcast(raw []byte) {
	msg, err := common.PubSubMessageFromJSON(raw)

	if err != nil {
		n.metrics.CounterIncrement(metricsUnknownBroadcast)
		n.log.Warnf("Failed to parse pubsub message '%s' with error: %v", raw, err)
		return
	}

	switch v := msg.(type) {
	case common.StreamMessage:
		n.broker.HandleBroadcast(&v)
	case common.RemoteCommandMessage:
		n.broker.HandleCommand(&v)
	}
}

// HandlePubSub parses incoming pubsub message and broadcast it to all clients (w/o using a broker)
func (n *Node) HandlePubSub(raw []byte) {
	msg, err := common.PubSubMessageFromJSON(raw)

	if err != nil {
		n.metrics.CounterIncrement(metricsUnknownBroadcast)
		n.log.Warnf("Failed to parse pubsub message '%s' with error: %v", raw, err)
		return
	}

	switch v := msg.(type) {
	case common.StreamMessage:
		n.Broadcast(&v)
	case common.RemoteCommandMessage:
		n.ExecuteRemoteCommand(&v)
	}
}

func (n *Node) LookupSession(id string) *Session {
	hubSession := n.hub.FindByIdentifier(id)
	session, _ := hubSession.(*Session)
	return session
}

// Shutdown stops all services (hub, controller)
func (n *Node) Shutdown() (err error) {
	n.shutdownMu.Lock()
	if n.closed {
		n.shutdownMu.Unlock()
		return errors.New("Already shut down")
	}

	close(n.shutdownCh)

	n.closed = true
	n.shutdownMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(n.config.ShutdownTimeout)*time.Second)
	defer cancel()

	if n.hub != nil {
		active := n.hub.Size()

		if active > 0 {
			n.log.Infof("Closing active connections: %d", active)
			n.disconnectAll(ctx)
			n.log.Info("All active connections closed")
		}

		n.hub.Shutdown()
	}

	if n.disconnector != nil {
		err := n.disconnector.Shutdown(ctx)

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

	return
}

func (n *Node) IsShuttingDown() bool {
	n.shutdownMu.Lock()
	defer n.shutdownMu.Unlock()

	return n.closed
}

type AuthOptions struct {
	DisconnectOnFailure bool
}

func newAuthOptions(modifiers []AuthOption) *AuthOptions {
	base := &AuthOptions{
		DisconnectOnFailure: true,
	}

	for _, modifier := range modifiers {
		modifier(base)
	}

	return base
}

type AuthOption = func(*AuthOptions)

func WithDisconnectOnFailure(disconnect bool) AuthOption {
	return func(opts *AuthOptions) {
		opts.DisconnectOnFailure = disconnect
	}
}

// Authenticate calls controller to perform authentication.
// If authentication is successful, session is registered with a hub.
func (n *Node) Authenticate(s *Session, options ...AuthOption) (res *common.ConnectResult, err error) {
	opts := newAuthOptions(options)

	restored := n.TryRestoreSession(s)

	if restored {
		return &common.ConnectResult{Status: common.SUCCESS}, nil
	}

	res, err = n.controller.Authenticate(s.GetID(), s.env)

	if err != nil {
		s.Disconnect("Auth Error", ws.CloseInternalServerErr)
		return
	}

	if res.Status == common.SUCCESS {
		n.Authenticated(s, res.Identifier)
	} else {
		if res.Status == common.FAILURE {
			n.metrics.CounterIncrement(metricsFailedAuths)
		}

		if opts.DisconnectOnFailure {
			defer s.Disconnect("Auth Failed", ws.CloseNormalClosure)
		}
	}

	n.handleCallReply(s, res.ToCallResult())
	n.markDisconnectable(s, res.DisconnectInterest)

	if berr := n.broker.CommitSession(s.GetID(), s); berr != nil {
		s.Log.Errorf("Failed to persist session in cache: %v", berr)
	}

	return
}

// Mark session as authenticated and register it with a hub.
// Useful when you perform authentication manually, not using a controller.
func (n *Node) Authenticated(s *Session, ids string) {
	s.SetIdentifiers(ids)
	s.Connected = true
	n.hub.AddSession(s)
}

func (n *Node) TryRestoreSession(s *Session) (restored bool) {
	sid := s.GetID()
	prev_sid := s.PrevSid()

	if prev_sid == "" {
		return false
	}

	cached_session, err := n.broker.RestoreSession(prev_sid)

	if err != nil {
		s.Log.Errorf("Failed to fetch session cache %s: %s", prev_sid, err.Error())
		return false
	}

	if cached_session == nil {
		s.Log.Debugf("Couldn't find session to restore from: %s", prev_sid)
		return false
	}

	err = s.RestoreFromCache(cached_session)

	if err != nil {
		s.Log.Errorf("Failed to restore session from cache %s: %s", prev_sid, err.Error())
		return false
	}

	s.Log.Debugf("Session restored from: %s", prev_sid)

	s.Connected = true
	n.hub.AddSession(s)

	// Resubscribe to streams
	for identifier, channel_streams := range s.subscriptions.channels {
		for stream := range channel_streams {
			streamId := n.broker.Subscribe(stream)
			n.hub.SubscribeSession(s, streamId, identifier)
		}
	}

	// Send welcome message
	s.Send(&common.Reply{
		Type:        common.WelcomeType,
		Sid:         sid,
		Restored:    true,
		RestoredIDs: utils.Keys(s.subscriptions.channels),
	})

	if berr := n.broker.CommitSession(s.GetID(), s); berr != nil {
		s.Log.Errorf("Failed to persist session in cache: %v", berr)
	}

	return true
}

// Subscribe subscribes session to a channel
func (n *Node) Subscribe(s *Session, msg *common.Message) (res *common.CommandResult, err error) {
	s.smu.Lock()

	if ok := s.subscriptions.HasChannel(msg.Identifier); ok {
		s.smu.Unlock()
		err = fmt.Errorf("Already subscribed to %s", msg.Identifier)
		return
	}

	res, err = n.controller.Subscribe(s.GetID(), s.env, s.GetIdentifiers(), msg.Identifier)

	var confirmed bool

	if err != nil { // nolint: gocritic
		if res == nil || res.Status == common.ERROR {
			s.Log.Errorf("Subscribe error: %v", err)
		}
	} else if res.Status == common.SUCCESS {
		confirmed = true
		s.subscriptions.AddChannel(msg.Identifier)
		s.Log.Debugf("Subscribed to channel: %s", msg.Identifier)
	} else {
		s.Log.Debugf("Subscription rejected: %s", msg.Identifier)
	}

	s.smu.Unlock()

	if res != nil {
		n.handleCommandReply(s, msg, res)
		n.markDisconnectable(s, res.DisconnectInterest)
	}

	if confirmed {
		if berr := n.broker.CommitSession(s.GetID(), s); berr != nil {
			s.Log.Errorf("Failed to persist session in cache: %v", berr)
		}

		if msg.History.Since > 0 || msg.History.Streams != nil {
			return res, n.History(s, msg)
		}
	}

	return
}

// Unsubscribe unsubscribes session from a channel
func (n *Node) Unsubscribe(s *Session, msg *common.Message) (res *common.CommandResult, err error) {
	s.smu.Lock()

	if ok := s.subscriptions.HasChannel(msg.Identifier); !ok {
		s.smu.Unlock()
		err = fmt.Errorf("Unknown subscription %s", msg.Identifier)
		return
	}

	res, err = n.controller.Unsubscribe(s.GetID(), s.env, s.GetIdentifiers(), msg.Identifier)

	if err != nil {
		if res == nil || res.Status == common.ERROR {
			s.Log.Errorf("Unsubscribe error: %v", err)
		}
	} else {
		// Make sure to remove all streams subscriptions
		res.StopAllStreams = true

		s.subscriptions.RemoveChannel(msg.Identifier)

		s.Log.Debugf("Unsubscribed from channel: %s", msg.Identifier)
	}

	s.smu.Unlock()

	if res != nil {
		n.handleCommandReply(s, msg, res)
	}

	if berr := n.broker.CommitSession(s.GetID(), s); berr != nil {
		s.Log.Errorf("Failed to persist session in cache: %v", berr)
	}

	return
}

// Perform executes client command
func (n *Node) Perform(s *Session, msg *common.Message) (res *common.CommandResult, err error) {
	s.smu.Lock()

	if ok := s.subscriptions.HasChannel(msg.Identifier); !ok {
		s.smu.Unlock()
		err = fmt.Errorf("Unknown subscription %s", msg.Identifier)
		return
	}

	s.smu.Unlock()

	data, ok := msg.Data.(string)

	if !ok {
		err = fmt.Errorf("Perform data must be a string, got %v", msg.Data)
		return
	}

	res, err = n.controller.Perform(s.GetID(), s.env, s.GetIdentifiers(), msg.Identifier, data)

	if err != nil {
		if res == nil || res.Status == common.ERROR {
			s.Log.Errorf("Perform error: %v", err)
		}
	} else {
		s.Log.Debugf("Perform result: %v", res)
	}

	if res != nil {
		if n.handleCommandReply(s, msg, res) {
			if berr := n.broker.CommitSession(s.GetID(), s); berr != nil {
				s.Log.Errorf("Failed to persist session in cache: %v", berr)
			}
		}
	}

	return
}

// History fetches the stream history for the specified identifier
func (n *Node) History(s *Session, msg *common.Message) (err error) {
	s.smu.Lock()

	if ok := s.subscriptions.HasChannel(msg.Identifier); !ok {
		s.smu.Unlock()
		err = fmt.Errorf("Unknown subscription %s", msg.Identifier)
		return
	}

	subscriptionStreams := s.subscriptions.StreamsFor(msg.Identifier)

	s.smu.Unlock()

	history := msg.History

	if history.Since == 0 && history.Streams == nil {
		err = fmt.Errorf("History request is missing, got %v", msg)
		return
	}

	backlog, err := n.retreiveHistory(&history, subscriptionStreams)

	if err != nil {
		s.Send(&common.Reply{
			Type:       common.HistoryRejectedType,
			Identifier: msg.Identifier,
		})

		return err
	}

	for _, el := range backlog {
		s.Send(el.ToReplyFor(msg.Identifier))
	}

	s.Send(&common.Reply{
		Type:       common.HistoryConfirmedType,
		Identifier: msg.Identifier,
	})

	return
}

func (n *Node) retreiveHistory(history *common.HistoryRequest, streams []string) (backlog []common.StreamMessage, err error) {
	backlog = []common.StreamMessage{}

	for _, stream := range streams {
		if history.Streams != nil {
			pos, ok := history.Streams[stream]

			if ok {
				streamBacklog, err := n.broker.HistoryFrom(stream, pos.Epoch, pos.Offset)

				if err != nil {
					return nil, err
				}

				backlog = append(backlog, streamBacklog...)

				continue
			}
		}

		if history.Since > 0 {
			streamBacklog, err := n.broker.HistorySince(stream, history.Since)

			if err != nil {
				return nil, err
			}

			backlog = append(backlog, streamBacklog...)
		}
	}

	return backlog, nil
}

// Broadcast message to stream (locally)
func (n *Node) Broadcast(msg *common.StreamMessage) {
	n.metrics.CounterIncrement(metricsBroadcastMsg)
	n.log.Debugf("Incoming broadcast message: %v", msg)
	n.hub.BroadcastMessage(msg)
}

// Execute remote command (locally)
func (n *Node) ExecuteRemoteCommand(msg *common.RemoteCommandMessage) {
	// TODO: Add remote commands metrics
	// n.metrics.CounterIncrement(metricsRemoteCommandsMsg)
	n.log.Debugf("Incoming remote command: %v", msg)

	switch msg.Command { // nolint:gocritic
	case "disconnect":
		dmsg, err := msg.ToRemoteDisconnectMessage()
		if err != nil {
			n.log.Warnf("Failed to parse remote disconnect command: %v", err)
			return
		}

		n.RemoteDisconnect(dmsg)
	}
}

// Disconnect adds session to disconnector queue and unregister session from hub
func (n *Node) Disconnect(s *Session) error {
	n.broker.FinishSession(s.GetID()) // nolint:errcheck

	if n.IsShuttingDown() {
		if s.IsDisconnectable() {
			return n.DisconnectNow(s)
		}
	} else {
		n.hub.RemoveSessionLater(s)

		if s.IsDisconnectable() {
			return n.disconnector.Enqueue(s)
		}
	}

	return nil
}

// DisconnectNow execute disconnect on controller
func (n *Node) DisconnectNow(s *Session) error {
	sessionSubscriptions := s.subscriptions.Channels()

	ids := s.GetIdentifiers()

	s.Log.Debugf("Disconnect %s %s %v %v", ids, s.env.URL, s.env.Headers, sessionSubscriptions)

	err := n.controller.Disconnect(
		s.GetID(),
		s.env,
		ids,
		sessionSubscriptions,
	)

	if err != nil {
		s.Log.Errorf("Disconnect error: %v", err)
	}

	return err
}

// RemoteDisconnect find a session by identifier and closes it
func (n *Node) RemoteDisconnect(msg *common.RemoteDisconnectMessage) {
	n.metrics.CounterIncrement(metricsBroadcastMsg)
	n.log.Debugf("Incoming pubsub command: %v", msg)
	n.hub.RemoteDisconnect(msg)
}

// Interest is represented as a int; -1 indicates no interest, 0 indicates lack of such information,
// and 1 indicates interest.
func (n *Node) markDisconnectable(s *Session, interest int) {
	switch n.config.DisconnectMode {
	case "always":
		s.MarkDisconnectable(true)
	case "never":
		s.MarkDisconnectable(false)
	case "auto":
		s.MarkDisconnectable(interest >= 0)
	}
}

func transmit(s *Session, transmissions []string) {
	for _, msg := range transmissions {
		s.SendJSONTransmission(msg)
	}
}

func (n *Node) handleCommandReply(s *Session, msg *common.Message, reply *common.CommandResult) bool {
	// Returns true if any of the subscriptions/channel/connections state has changed
	isDirty := false

	if reply.Disconnect {
		defer s.Disconnect("Command Failed", ws.CloseAbnormalClosure)
	}

	if reply.StopAllStreams {
		n.hub.UnsubscribeSessionFromChannel(s, msg.Identifier)
		removedStreams := s.subscriptions.RemoveChannelStreams(msg.Identifier)

		for _, stream := range removedStreams {
			isDirty = true
			n.broker.Unsubscribe(stream)
		}

	} else if reply.StoppedStreams != nil {
		isDirty = true

		for _, stream := range reply.StoppedStreams {
			streamId := n.broker.Unsubscribe(stream)
			n.hub.UnsubscribeSession(s, streamId, msg.Identifier)
			s.subscriptions.RemoveChannelStream(msg.Identifier, streamId)
		}
	}

	if reply.Streams != nil {
		isDirty = true

		for _, stream := range reply.Streams {
			streamId := n.broker.Subscribe(stream)
			n.hub.SubscribeSession(s, streamId, msg.Identifier)
			s.subscriptions.AddChannelStream(msg.Identifier, streamId)
		}
	}

	if reply.IState != nil {
		isDirty = true

		s.smu.Lock()
		s.env.MergeChannelState(msg.Identifier, &reply.IState)
		s.smu.Unlock()
	}

	isConnectionDirty := n.handleCallReply(s, reply.ToCallResult())
	return isDirty || isConnectionDirty
}

func (n *Node) handleCallReply(s *Session, reply *common.CallResult) bool {
	isDirty := false

	if reply.CState != nil {
		isDirty = true

		s.smu.Lock()
		s.env.MergeConnectionState(&reply.CState)
		s.smu.Unlock()
	}

	if reply.Broadcasts != nil {
		for _, broadcast := range reply.Broadcasts {
			n.Broadcast(broadcast)
		}
	}

	if reply.Transmissions != nil {
		transmit(s, reply.Transmissions)
	}

	return isDirty
}

func (n *Node) disconnectAll(ctx context.Context) {
	disconnectMessage := common.NewDisconnectMessage(common.SERVER_RESTART_REASON, true)

	ok := n.hub.DisconnectSesssions(ctx, func(s hub.HubSession) {
		s.DisconnectWithMessage(disconnectMessage, common.SERVER_RESTART_REASON)
	})

	if !ok {
		n.log.Warnf("Timed out to disconnect all sessions, left: %d", n.hub.Size())
	}
}

func (n *Node) collectStats() {
	statsCollectInterval := time.Duration(n.config.StatsRefreshInterval) * time.Second

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
	n.metrics.GaugeSet(metricsGoroutines, uint64(runtime.NumGoroutine()))

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	n.metrics.GaugeSet(metricsMemSys, m.Sys)

	n.metrics.GaugeSet(metricsClientsNum, uint64(n.hub.Size()))
	n.metrics.GaugeSet(metricsUniqClientsNum, uint64(n.hub.UniqSize()))
	n.metrics.GaugeSet(metricsStreamsNum, uint64(n.hub.StreamsSize()))
	n.metrics.GaugeSet(metricsDisconnectQueue, uint64(n.disconnector.Size()))
}

func (n *Node) registerMetrics() {
	n.metrics.RegisterGauge(metricsGoroutines, "The number of Go routines")
	n.metrics.RegisterGauge(metricsMemSys, "The total bytes of memory obtained from the OS")

	n.metrics.RegisterGauge(metricsClientsNum, "The number of active clients")
	n.metrics.RegisterGauge(metricsUniqClientsNum, "The number of unique clients (with respect to connection identifiers)")
	n.metrics.RegisterGauge(metricsStreamsNum, "The number of active broadcasting streams")
	n.metrics.RegisterGauge(metricsDisconnectQueue, "The size of delayed disconnect")

	n.metrics.RegisterCounter(metricsFailedAuths, "The total number of failed authentication attempts")
	n.metrics.RegisterCounter(metricsReceivedMsg, "The total number of received messages from clients")
	n.metrics.RegisterCounter(metricsFailedCommandReceived, "The total number of unrecognized messages received from clients")
	n.metrics.RegisterCounter(metricsBroadcastMsg, "The total number of messages received through PubSub (for broadcast)")
	n.metrics.RegisterCounter(metricsUnknownBroadcast, "The total number of unrecognized messages received through PubSub")

	n.metrics.RegisterCounter(metricsSentMsg, "The total number of messages sent to clients")
	n.metrics.RegisterCounter(metricsFailedSent, "The total number of messages failed to send to clients")

	n.metrics.RegisterCounter(metricsDataSent, "The total amount of bytes sent to clients")
	n.metrics.RegisterCounter(metricsDataReceived, "The total amount of bytes received from clients")
}
