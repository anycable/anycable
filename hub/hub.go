package hub

import (
	"context"
	"hash/fnv"
	"log/slog"
	"sync"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/utils"
)

type HubSession interface {
	GetID() string
	GetIdentifiers() string
	Send(msg encoders.EncodedMessage)
	DisconnectWithMessage(msg encoders.EncodedMessage, code string)
}

// HubRegistration represents registration event ("add" or "remove")
type HubRegistration struct {
	event   string
	session HubSession
}

// HubSessionInfo is used to track registered sessions
type HubSessionInfo struct {
	session HubSession
	// List of stream-identifier pairs
	streams [][]string
}

func NewHubSessionInfo(session HubSession) *HubSessionInfo {
	return &HubSessionInfo{
		session: session,
		streams: make([][]string, 0),
	}
}

func (hs *HubSessionInfo) AddStream(stream string, identifier string) {
	hs.streams = append(hs.streams, []string{stream, identifier})
}

func (hs *HubSessionInfo) RemoveStream(stream string, identifier string) {
	for i, s := range hs.streams {
		if s[0] == stream && s[1] == identifier {
			hs.streams = append(hs.streams[:i], hs.streams[i+1:]...)
			break
		}
	}
}

// Hub stores all the sessions and the corresponding subscriptions info
type Hub struct {
	// Gates (=shards)
	gates    []*Gate
	gatesNum int

	// Registered sessions
	sessions map[string]*HubSessionInfo

	// Identifiers to session
	identifiers map[string]map[string]bool

	// Messages for specified stream
	broadcast chan *common.StreamMessage

	// Remote disconnect commands
	disconnect chan *common.RemoteDisconnectMessage

	// Register requests from the sessions
	register chan HubRegistration

	// Control channel to shutdown hub
	shutdown chan struct{}

	// Synchronization group to wait for gracefully disconnect of all sessions
	done sync.WaitGroup

	doneFn context.CancelFunc

	// Log context
	log *slog.Logger

	// go pool
	pool *utils.GoPool

	// mutex for sessions data tracking
	mu sync.RWMutex
}

// NewHub builds new hub instance
func NewHub(poolSize int) *Hub {
	ctx, doneFn := context.WithCancel(context.Background())

	return &Hub{
		broadcast:   make(chan *common.StreamMessage, 256),
		disconnect:  make(chan *common.RemoteDisconnectMessage, 128),
		register:    make(chan HubRegistration, 2048),
		sessions:    make(map[string]*HubSessionInfo),
		identifiers: make(map[string]map[string]bool),
		gates:       buildGates(ctx, poolSize),
		gatesNum:    poolSize,
		pool:        utils.NewGoPool("remote commands", 256),
		doneFn:      doneFn,
		shutdown:    make(chan struct{}),
		log:         slog.With("context", "hub"),
	}
}

// Run makes hub active
func (h *Hub) Run() {
	h.done.Add(1)
	for {
		select {
		case r := <-h.register:
			if r.event == "add" {
				h.AddSession(r.session)
			} else {
				h.RemoveSession(r.session)
			}

		case message := <-h.broadcast:
			h.broadcastToStream(message)

		case command := <-h.disconnect:
			h.disconnectSessions(command.Identifier, command.Reconnect)

		case <-h.shutdown:
			h.done.Done()
			return
		}
	}
}

// RemoveSession enqueues session un-registration
func (h *Hub) RemoveSessionLater(s HubSession) {
	h.register <- HubRegistration{event: "remove", session: s}
}

// Broadcast enqueues data broadcasting to a stream
func (h *Hub) Broadcast(stream string, data string) {
	h.broadcast <- &common.StreamMessage{Stream: stream, Data: data}
}

// BroadcastMessage enqueues broadcasting a pre-built StreamMessage
func (h *Hub) BroadcastMessage(msg *common.StreamMessage) {
	h.broadcast <- msg
}

// RemoteDisconnect enqueues remote disconnect command
func (h *Hub) RemoteDisconnect(msg *common.RemoteDisconnectMessage) {
	h.disconnect <- msg
}

// Shutdown sends shutdown command to hub
func (h *Hub) Shutdown() {
	h.shutdown <- struct{}{}

	// Wait for stop listening channels
	h.done.Wait()
}

// Size returns a number of active sessions
func (h *Hub) Size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.sessions)
}

// UniqSize returns a number of uniq identifiers
func (h *Hub) UniqSize() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.identifiers)
}

// StreamsSize returns a number of uniq streams
func (h *Hub) StreamsSize() int {
	size := 0
	for _, gate := range h.gates {
		size += gate.Size()
	}
	return size
}

func (h *Hub) AddSession(session HubSession) {
	h.mu.Lock()
	defer h.mu.Unlock()

	uid := session.GetID()
	identifiers := session.GetIdentifiers()

	h.sessions[uid] = NewHubSessionInfo(session)

	if _, ok := h.identifiers[identifiers]; !ok {
		h.identifiers[identifiers] = make(map[string]bool)
	}

	h.identifiers[identifiers][uid] = true

	h.log.With("sid", uid).Debug(
		"registered", "ids", identifiers,
	)
}

func (h *Hub) RemoveSession(session HubSession) {
	h.mu.RLock()
	uid := session.GetID()

	if _, ok := h.sessions[uid]; !ok {
		h.mu.RUnlock()
		h.log.With("sid", uid).Warn("session hasn't been registered")
		return
	}
	h.mu.RUnlock()

	identifiers := session.GetIdentifiers()
	h.unsubscribeSessionFromAllChannels(session)

	h.mu.Lock()

	delete(h.sessions, uid)
	delete(h.identifiers[identifiers], uid)

	if len(h.identifiers[identifiers]) == 0 {
		delete(h.identifiers, identifiers)
	}

	h.mu.Unlock()

	h.log.With("sid", uid).Debug("unregistered")
}

func (h *Hub) unsubscribeSessionFromAllChannels(session HubSession) {
	h.mu.Lock()
	defer h.mu.Unlock()

	sid := session.GetID()

	if sessionInfo, ok := h.sessions[sid]; ok {
		for _, streamInfo := range sessionInfo.streams {
			stream, identifier := streamInfo[0], streamInfo[1]

			h.gates[index(stream, h.gatesNum)].Unsubscribe(session, stream, identifier)
		}
	}
}

func (h *Hub) UnsubscribeSessionFromChannel(session HubSession, targetIdentifier string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	sid := session.GetID()

	if sessionInfo, ok := h.sessions[sid]; ok {
		for _, streamInfo := range sessionInfo.streams {
			stream, identifier := streamInfo[0], streamInfo[1]

			if targetIdentifier == identifier {
				h.gates[index(stream, h.gatesNum)].Unsubscribe(session, stream, identifier)
				sessionInfo.RemoveStream(stream, identifier)
			}
		}
	}

	h.log.With("sid", sid).Debug("unsubscribed", "channel", targetIdentifier)
}

func (h *Hub) SubscribeSession(session HubSession, stream string, identifier string) {
	h.gates[index(stream, h.gatesNum)].Subscribe(session, stream, identifier)

	h.mu.Lock()
	defer h.mu.Unlock()

	sid := session.GetID()

	if _, ok := h.sessions[sid]; !ok {
		h.sessions[sid] = NewHubSessionInfo(session)
	}

	h.sessions[sid].AddStream(stream, identifier)

	h.log.With("sid", sid).Debug("subscribed", "channel", identifier, "stream", stream)
}

func (h *Hub) UnsubscribeSession(session HubSession, stream string, identifier string) {
	h.gates[index(stream, h.gatesNum)].Unsubscribe(session, stream, identifier)

	h.mu.Lock()
	defer h.mu.Unlock()

	sid := session.GetID()

	if info, ok := h.sessions[sid]; ok {
		info.RemoveStream(stream, identifier)
	}

	h.log.With("sid", sid).Debug("unsubscribed", "channel", identifier, "stream", stream)
}

func (h *Hub) broadcastToStream(streamMsg *common.StreamMessage) {
	h.gates[index(streamMsg.Stream, h.gatesNum)].Broadcast(streamMsg)
}

func (h *Hub) disconnectSessions(identifier string, reconnect bool) {
	h.mu.RLock()
	ids, ok := h.identifiers[identifier]
	h.mu.RUnlock()

	if !ok {
		h.log.Debug("cannot disconnect session", "identifier", identifier, "reason", "not found")
		return
	}

	msg := common.NewDisconnectMessage(common.REMOTE_DISCONNECT_REASON, reconnect)

	h.pool.Schedule(func() {
		h.mu.RLock()
		defer h.mu.RUnlock()

		for id := range ids {
			if sinfo, ok := h.sessions[id]; ok {
				sinfo.session.DisconnectWithMessage(msg, common.REMOTE_DISCONNECT_REASON)
			}
		}
	})
}

func (h *Hub) FindByIdentifier(id string) HubSession {
	h.mu.RLock()
	defer h.mu.RUnlock()

	ids, ok := h.identifiers[id]

	if !ok {
		return nil
	}

	for id := range ids {
		if info, ok := h.sessions[id]; ok {
			return info.session
		}
	}

	return nil
}

func (h *Hub) Sessions() []HubSession {
	h.mu.RLock()
	defer h.mu.RUnlock()

	sessions := make([]HubSession, 0, len(h.sessions))

	for _, info := range h.sessions {
		sessions = append(sessions, info.session)
	}

	return sessions
}

func buildGates(ctx context.Context, num int) []*Gate {
	gates := make([]*Gate, 0, num)
	for i := 0; i < num; i++ {
		gates = append(gates, NewGate(ctx))
	}

	return gates
}

func index(stream string, size int) int {
	if size == 1 {
		return 0
	}

	hash := fnv.New64a()
	hash.Write([]byte(stream))
	return int(hash.Sum64() % uint64(size))
}
