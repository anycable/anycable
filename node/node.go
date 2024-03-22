package node

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/anycable/anycable-go/broker"
	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/hub"
	"github.com/anycable/anycable-go/logger"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/utils"
	"github.com/anycable/anycable-go/ws"
	"github.com/joomcode/errorx"
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
	id      string
	metrics metrics.Instrumenter

	config       *Config
	hub          *hub.Hub
	broker       broker.Broker
	controller   Controller
	disconnector Disconnector
	shutdownCh   chan struct{}
	shutdownMu   sync.Mutex
	closed       bool
	log          *slog.Logger
}

var _ AppNode = (*Node)(nil)

type NodeOption = func(*Node)

func WithController(c Controller) NodeOption {
	return func(n *Node) {
		n.controller = c
	}
}

func WithInstrumenter(i metrics.Instrumenter) NodeOption {
	return func(n *Node) {
		n.metrics = i
	}
}

func WithLogger(l *slog.Logger) NodeOption {
	return func(n *Node) {
		n.log = l.With("context", "node")
	}
}

func WithID(id string) NodeOption {
	return func(n *Node) {
		n.id = id
	}
}

// NewNode builds new node struct
func NewNode(config *Config, opts ...NodeOption) *Node {
	n := &Node{
		config:     config,
		shutdownCh: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(n)
	}

	// Setup default logger
	if n.log == nil {
		n.log = slog.With("context", "node")
	}

	n.hub = hub.NewHub(config.HubGopoolSize, n.log)

	if n.metrics != nil {
		n.registerMetrics()
	}

	return n
}

// Start runs all the required goroutines
func (n *Node) Start() error {
	go n.hub.Run()
	go n.collectStats()

	return nil
}

// ID returns node identifier
func (n *Node) ID() string {
	return n.id
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
	s.Log.Debug("incoming message", "data", msg)
	switch msg.Command {
	case "pong":
		s.handlePong(msg)
	case "subscribe":
		_, err = n.Subscribe(s, msg)
	case "unsubscribe":
		_, err = n.Unsubscribe(s, msg)
	case "message":
		_, err = n.Perform(s, msg)
	case "history":
		err = n.History(s, msg)
	case "whisper":
		err = n.Whisper(s, msg)
	default:
		err = fmt.Errorf("unknown command: %s", msg.Command)
	}

	return
}

// HandleBroadcast parses incoming broadcast message, record it and re-transmit to other nodes
func (n *Node) HandleBroadcast(raw []byte) {
	msg, err := common.PubSubMessageFromJSON(raw)

	if err != nil {
		n.metrics.CounterIncrement(metricsUnknownBroadcast)
		n.log.Warn("failed to parse pubsub message", "data", logger.CompactValue(raw), "error", err)
		return
	}

	switch v := msg.(type) {
	case common.StreamMessage:
		n.log.Debug("handle broadcast message", "payload", &v)
		n.broker.HandleBroadcast(&v)
	case []*common.StreamMessage:
		n.log.Debug("handle batch-broadcast message", "payload", &v)
		for _, el := range v {
			n.broker.HandleBroadcast(el)
		}
	case common.RemoteCommandMessage:
		n.log.Debug("handle remote command", "command", &v)
		n.broker.HandleCommand(&v)
	}
}

// HandlePubSub parses incoming pubsub message and broadcast it to all clients (w/o using a broker)
func (n *Node) HandlePubSub(raw []byte) {
	msg, err := common.PubSubMessageFromJSON(raw)

	if err != nil {
		n.metrics.CounterIncrement(metricsUnknownBroadcast)
		n.log.Warn("failed to parse pubsub message", "data", logger.CompactValue(raw), "error", err)
		return
	}

	switch v := msg.(type) {
	case common.StreamMessage:
		n.Broadcast(&v)
	case []*common.StreamMessage:
		for _, el := range v {
			n.Broadcast(el)
		}
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
func (n *Node) Shutdown(ctx context.Context) (err error) {
	n.shutdownMu.Lock()
	if n.closed {
		n.shutdownMu.Unlock()
		return errors.New("already shut down")
	}

	close(n.shutdownCh)

	n.closed = true
	n.shutdownMu.Unlock()

	if n.hub != nil {
		active := n.hub.Size()

		if active > 0 {
			n.log.Info("closing active connections", "num", active)
			n.disconnectAll(ctx)
		}

		n.hub.Shutdown()
	}

	if n.disconnector != nil {
		err := n.disconnector.Shutdown(ctx)

		if err != nil {
			n.log.Warn("failed to shutdown disconnector gracefully", "error", err)
		}
	}

	if n.controller != nil {
		err := n.controller.Shutdown()

		if err != nil {
			n.log.Warn("failed to shutdown controller gracefully", "error", err)
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
func (n *Node) Authenticate(s *Session, options ...AuthOption) (*common.ConnectResult, error) {
	opts := newAuthOptions(options)

	if s.IsResumeable() {
		restored := n.TryRestoreSession(s)

		if restored {
			return &common.ConnectResult{Status: common.SUCCESS}, nil
		}
	}

	res, err := n.controller.Authenticate(s.GetID(), s.env)

	s.Log.Debug("controller authenticate", "response", res, "err", err)

	if err != nil {
		s.Disconnect("Auth Error", ws.CloseInternalServerErr)
		return nil, errorx.Decorate(err, "failed to authenticate")
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

	if s.IsResumeable() {
		if berr := n.broker.CommitSession(s.GetID(), s); berr != nil {
			s.Log.Error("failed to persist session in cache", "error", berr)
		}
	}

	return res, nil
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
		s.Log.Error("failed to fetch session cache", "old_sid", prev_sid, "error", err)
		return false
	}

	if cached_session == nil {
		s.Log.Debug("session not found in cache", "old_sid", prev_sid)
		return false
	}

	err = s.RestoreFromCache(cached_session)

	if err != nil {
		s.Log.Error("failed to restore session from cache", "old_sid", prev_sid, "error", err)
		return false
	}

	s.Log.Debug("session restored", "old_sid", prev_sid)

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

	if s.IsResumeable() {
		if berr := n.broker.CommitSession(s.GetID(), s); berr != nil {
			s.Log.Error("failed to persist session in cache", "error", berr)
		}
	}

	return true
}

// Subscribe subscribes session to a channel
func (n *Node) Subscribe(s *Session, msg *common.Message) (*common.CommandResult, error) {
	s.smu.Lock()

	if ok := s.subscriptions.HasChannel(msg.Identifier); ok {
		s.smu.Unlock()
		return nil, fmt.Errorf("already subscribed to %s", msg.Identifier)
	}

	res, err := n.controller.Subscribe(s.GetID(), s.env, s.GetIdentifiers(), msg.Identifier)

	s.Log.Debug("controller subscribe", "response", res, "err", err)

	var confirmed bool

	if err != nil { // nolint: gocritic
		if res == nil || res.Status == common.ERROR {
			return nil, errorx.Decorate(err, "subscribe failed for %s", msg.Identifier)
		}
	} else if res.Status == common.SUCCESS {
		confirmed = true
		s.subscriptions.AddChannel(msg.Identifier)
		s.Log.Debug("subscribed", "identifier", msg.Identifier)
	} else {
		s.Log.Debug("subscription rejected", "identifier", msg.Identifier)
	}

	s.smu.Unlock()

	if res != nil {
		n.handleCommandReply(s, msg, res)
		n.markDisconnectable(s, res.DisconnectInterest)
	}

	if confirmed {
		if s.IsResumeable() {
			if berr := n.broker.CommitSession(s.GetID(), s); berr != nil {
				s.Log.Error("failed to persist session in cache", "error", berr)
			}
		}

		if msg.History.Since > 0 || msg.History.Streams != nil {
			if err := n.History(s, msg); err != nil {
				s.Log.Warn("couldn't retrieve history", "identifier", msg.Identifier, "error", err)
			}

			return res, nil
		}
	}

	return res, nil
}

// Unsubscribe unsubscribes session from a channel
func (n *Node) Unsubscribe(s *Session, msg *common.Message) (*common.CommandResult, error) {
	s.smu.Lock()

	if ok := s.subscriptions.HasChannel(msg.Identifier); !ok {
		s.smu.Unlock()
		return nil, fmt.Errorf("unknown subscription: %s", msg.Identifier)
	}

	res, err := n.controller.Unsubscribe(s.GetID(), s.env, s.GetIdentifiers(), msg.Identifier)

	s.Log.Debug("controller unsubscribe", "response", res, "err", err)

	if err != nil {
		if res == nil || res.Status == common.ERROR {
			return nil, errorx.Decorate(err, "failed to unsubscribe from %s", msg.Identifier)
		}
	} else {
		// Make sure to remove all streams subscriptions
		res.StopAllStreams = true

		s.env.RemoveChannelState(msg.Identifier)
		s.subscriptions.RemoveChannel(msg.Identifier)

		s.Log.Debug("unsubscribed", "identifier", msg.Identifier)
	}

	s.smu.Unlock()

	if res != nil {
		n.handleCommandReply(s, msg, res)
	}

	if s.IsResumeable() {
		if berr := n.broker.CommitSession(s.GetID(), s); berr != nil {
			s.Log.Error("failed to persist session in cache", "error", berr)
		}
	}

	return res, nil
}

// Perform executes client command
func (n *Node) Perform(s *Session, msg *common.Message) (*common.CommandResult, error) {
	s.smu.Lock()

	if ok := s.subscriptions.HasChannel(msg.Identifier); !ok {
		s.smu.Unlock()
		return nil, fmt.Errorf("unknown subscription %s", msg.Identifier)
	}

	s.smu.Unlock()

	data, ok := msg.Data.(string)

	if !ok {
		return nil, fmt.Errorf("perform data must be a string, got %v", msg.Data)
	}

	res, err := n.controller.Perform(s.GetID(), s.env, s.GetIdentifiers(), msg.Identifier, data)

	s.Log.Debug("controller perform", "response", res, "err", err)

	if err != nil {
		if res == nil || res.Status == common.ERROR {
			return nil, errorx.Decorate(err, "perform failed for %s", msg.Identifier)
		}
	}

	if res != nil {
		if n.handleCommandReply(s, msg, res) {
			if s.IsResumeable() {
				if berr := n.broker.CommitSession(s.GetID(), s); berr != nil {
					s.Log.Error("failed to persist session in cache", "error", berr)
				}
			}
		}
	}

	return res, nil
}

// History fetches the stream history for the specified identifier
func (n *Node) History(s *Session, msg *common.Message) error {
	s.smu.Lock()

	if ok := s.subscriptions.HasChannel(msg.Identifier); !ok {
		s.smu.Unlock()
		return fmt.Errorf("unknown subscription %s", msg.Identifier)
	}

	subscriptionStreams := s.subscriptions.StreamsFor(msg.Identifier)

	s.smu.Unlock()

	history := msg.History

	if history.Since == 0 && history.Streams == nil {
		return fmt.Errorf("history request is missing, got %v", msg)
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

	return nil
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

// Whisper broadcasts the message to the specified whispering stream to
// all clients except the sender
func (n *Node) Whisper(s *Session, msg *common.Message) error {
	// The session must have the whisper stream name defined in the state to be able to whisper
	// If the stream is not defined, the whisper message is ignored
	env := s.GetEnv()
	if env == nil {
		return errors.New("session environment is missing")
	}

	stream := env.GetChannelStateField(msg.Identifier, common.WHISPER_STREAM_STATE)

	if stream == "" {
		s.Log.Debug("whisper stream not found", "identifier", msg.Identifier)
		return nil
	}

	broadcast := &common.StreamMessage{
		Stream: stream,
		Data:   string(utils.ToJSON(msg.Data)),
		Meta: &common.StreamMessageMetadata{
			ExcludeSocket: s.GetID(),
			BroadcastType: common.WhisperType,
			Transient:     true,
		},
	}

	n.broker.HandleBroadcast(broadcast)

	s.Log.Debug("whispered", "stream", stream)

	return nil
}

// Broadcast message to stream (locally)
func (n *Node) Broadcast(msg *common.StreamMessage) {
	n.metrics.CounterIncrement(metricsBroadcastMsg)
	n.log.Debug("incoming broadcast message", "payload", msg)
	n.hub.BroadcastMessage(msg)
}

// Execute remote command (locally)
func (n *Node) ExecuteRemoteCommand(msg *common.RemoteCommandMessage) {
	// TODO: Add remote commands metrics
	// n.metrics.CounterIncrement(metricsRemoteCommandsMsg)
	switch msg.Command { // nolint:gocritic
	case "disconnect":
		dmsg, err := msg.ToRemoteDisconnectMessage()
		if err != nil {
			n.log.Warn("failed to parse remote disconnect command", "data", msg, "error", err)
			return
		}

		n.log.Debug("incoming remote command", "command", dmsg)

		n.RemoteDisconnect(dmsg)
	}
}

// Disconnect adds session to disconnector queue and unregister session from hub
func (n *Node) Disconnect(s *Session) error {
	if s.IsResumeable() {
		n.broker.FinishSession(s.GetID()) // nolint:errcheck
	}

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

	s.Log.Debug("disconnect", "ids", ids, "url", s.env.URL, "headers", s.env.Headers, "subscriptions", sessionSubscriptions)

	err := n.controller.Disconnect(
		s.GetID(),
		s.env,
		ids,
		sessionSubscriptions,
	)

	if err != nil {
		s.Log.Error("controller disconnect failed", "error", err)
	}

	s.Log.Debug("controller disconnect succeeded")

	return err
}

// RemoteDisconnect find a session by identifier and closes it
func (n *Node) RemoteDisconnect(msg *common.RemoteDisconnectMessage) {
	n.metrics.CounterIncrement(metricsBroadcastMsg)
	n.log.Debug("incoming pubsub command", "data", msg)
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

func (n *Node) Size() int {
	return n.hub.Size()
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
			n.broker.HandleBroadcast(broadcast)
		}
	}

	if reply.Transmissions != nil {
		transmit(s, reply.Transmissions)
	}

	return isDirty
}

// disconnectScheduler controls how quickly to disconnect sessions
type disconnectScheduler interface {
	// This method is called when a session is ready to be disconnected,
	// so it can block the operation or cancel it (by returning false).
	Continue() bool
}

type noopScheduler struct {
	ctx context.Context
}

func (s *noopScheduler) Continue() bool {
	return s.ctx.Err() == nil
}

func (n *Node) disconnectAll(ctx context.Context) {
	disconnectMessage := common.NewDisconnectMessage(common.SERVER_RESTART_REASON, true)

	// To speed up the process we disconnect all sessions in parallel using a pool of workers
	pool := utils.NewGoPool("disconnect", n.config.ShutdownDisconnectPoolSize)

	sessions := n.hub.Sessions()

	var scheduler disconnectScheduler // nolint:gosimple

	scheduler = &noopScheduler{ctx}

	var wg sync.WaitGroup

	wg.Add(len(sessions))

	for _, s := range sessions {
		s := s.(*Session)
		pool.Schedule(func() {
			if scheduler.Continue() {
				if s.IsConnected() {
					s.DisconnectWithMessage(disconnectMessage, common.SERVER_RESTART_REASON)
				}
				wg.Done()
			}
		})
	}

	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		n.log.Warn("terminated while disconnecting active sessions", "num", n.hub.Size())
	case <-done:
		n.log.Info("all active connections closed")
	}
}

func (n *Node) collectStats() {
	if n.config.StatsRefreshInterval == 0 {
		return
	}

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
