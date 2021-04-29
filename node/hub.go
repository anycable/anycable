package node

import (
	"encoding/json"
	"sync"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/utils"
	"github.com/apex/log"
)

// HubSubscription contains information about session-channel(-stream) subscription
type HubSubscription struct {
	event      string
	session    string
	stream     string
	identifier string
}

// HubRegistration represents registration event ("add" or "remove")
type HubRegistration struct {
	event   string
	session *Session
}

// Hub stores all the sessions and the corresponding subscriptions info
type Hub struct {
	// Registered sessions
	sessions map[string]*Session

	// Identifiers to session
	identifiers map[string]map[string]bool

	// Maps streams to sessions with identifiers
	// stream -> sid -> identifier -> true
	streams map[string]map[string]map[string]bool

	// Maps sessions to identifiers to streams
	// sid -> identifier -> [stream]
	sessionsStreams map[string]map[string][]string

	// Messages for specified stream
	broadcast chan *common.StreamMessage

	// Remote disconnect commands
	disconnect chan *common.RemoteDisconnectMessage

	// Register requests from the sessions
	register chan HubRegistration

	// Subscribe requests to streams
	subscribe chan HubSubscription

	// Control channel to shutdown hub
	shutdown chan struct{}

	// Synchronization group to wait for gracefully disconnect of all sessions
	done sync.WaitGroup

	// Log context
	log *log.Entry

	// go pool
	pool *utils.GoPool

	// mutex
	mu sync.RWMutex
}

// NewHub builds new hub instance
func NewHub(poolSize int) *Hub {
	return &Hub{
		broadcast:       make(chan *common.StreamMessage, 256),
		disconnect:      make(chan *common.RemoteDisconnectMessage, 128),
		register:        make(chan HubRegistration, 2048),
		subscribe:       make(chan HubSubscription, 128),
		sessions:        make(map[string]*Session),
		identifiers:     make(map[string]map[string]bool),
		streams:         make(map[string]map[string]map[string]bool),
		sessionsStreams: make(map[string]map[string][]string),
		shutdown:        make(chan struct{}),
		log:             log.WithFields(log.Fields{"context": "hub"}),
		pool:            utils.NewGoPool(poolSize),
	}
}

// Run makes hub active
func (h *Hub) Run() {
	h.done.Add(1)
	for {
		select {
		case r := <-h.register:
			if r.event == "add" {
				h.addSession(r.session)
			} else {
				h.removeSession(r.session)
			}

		case subinfo := <-h.subscribe:
			if subinfo.event == "add" {
				h.subscribeSession(subinfo.session, subinfo.stream, subinfo.identifier)
			} else if subinfo.event == "removeAll" {
				h.unsubscribeSessionFromChannel(subinfo.session, subinfo.identifier)
			} else {
				h.unsubscribeSession(subinfo.session, subinfo.stream, subinfo.identifier)
			}

		case message := <-h.broadcast:
			h.broadcastToStream(message.Stream, message.Data)

		case command := <-h.disconnect:
			h.disconnectSessions(command.Identifier, command.Reconnect)

		case <-h.shutdown:
			h.done.Done()
			return
		}
	}
}

// AddSession enqueues sessions registration
func (h *Hub) AddSession(s *Session) {
	h.register <- HubRegistration{event: "add", session: s}
}

// RemoveSession enqueues session un-registration
func (h *Hub) RemoveSession(s *Session) {
	h.register <- HubRegistration{event: "remove", session: s}
}

// AddSubscription enqueues adding a subscription for session-identifier pair to the hub
func (h *Hub) AddSubscription(sid string, identifier string, stream string) {
	h.subscribe <- HubSubscription{event: "add", session: sid, identifier: identifier, stream: stream}
}

// RemoveSubscription enqueues removing a subscription for session-identifier pair from the hub
func (h *Hub) RemoveSubscription(sid string, identifier string, stream string) {
	h.subscribe <- HubSubscription{event: "remove", session: sid, identifier: identifier, stream: stream}
}

// RemoveAllSubscriptions enqueues removing all subscription for session-identifier pair from the hub
func (h *Hub) RemoveAllSubscriptions(sid string, identifier string) {
	h.subscribe <- HubSubscription{event: "removeAll", session: sid, identifier: identifier}
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
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.streams)
}

func (h *Hub) addSession(session *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.sessions[session.UID] = session

	if _, ok := h.identifiers[session.Identifiers]; !ok {
		h.identifiers[session.Identifiers] = make(map[string]bool)
	}

	h.identifiers[session.Identifiers][session.UID] = true

	h.log.WithField("sid", session.UID).Debugf(
		"Registered with identifiers: %s",
		session.Identifiers,
	)
}

func (h *Hub) removeSession(session *Session) {
	if _, ok := h.sessions[session.UID]; !ok {
		h.log.WithField("sid", session.UID).Warn("Session hasn't been registered")
		return
	}

	h.unsubscribeSessionFromAllChannels(session.UID)

	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.sessions, session.UID)
	delete(h.identifiers[session.Identifiers], session.UID)

	if len(h.identifiers[session.Identifiers]) == 0 {
		delete(h.identifiers, session.Identifiers)
	}

	h.log.WithField("sid", session.UID).Debug("Unregistered")
}

func (h *Hub) unsubscribeSessionFromAllChannels(sid string) {
	for channel := range h.sessionsStreams[sid] {
		h.unsubscribeSessionFromChannel(sid, channel)
	}

	delete(h.sessionsStreams, sid)
}

func (h *Hub) unsubscribeSessionFromChannel(sid string, identifier string) {
	if _, ok := h.sessionsStreams[sid]; !ok {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, stream := range h.sessionsStreams[sid][identifier] {
		delete(h.streams[stream][sid], identifier)

		if len(h.streams[stream][sid]) == 0 {
			delete(h.streams[stream], sid)
		}

		if len(h.streams[stream]) == 0 {
			delete(h.streams, stream)
		}
	}

	h.log.WithFields(log.Fields{
		"sid":     sid,
		"channel": identifier,
	}).Debug("Unsubscribed")
}

func (h *Hub) subscribeSession(sid string, stream string, identifier string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.streams[stream]; !ok {
		h.streams[stream] = make(map[string]map[string]bool)
	}

	if _, ok := h.streams[stream][sid]; !ok {
		h.streams[stream][sid] = make(map[string]bool)
	}

	h.streams[stream][sid][identifier] = true

	if _, ok := h.sessionsStreams[sid]; !ok {
		h.sessionsStreams[sid] = make(map[string][]string)
	}

	h.sessionsStreams[sid][identifier] = append(
		h.sessionsStreams[sid][identifier],
		stream,
	)

	h.log.WithFields(log.Fields{
		"sid":     sid,
		"channel": identifier,
		"stream":  stream,
	}).Debug("Subscribed")
}

func (h *Hub) unsubscribeSession(sid string, stream string, identifier string) {
	if _, ok := h.streams[stream]; !ok {
		return
	}

	if _, ok := h.streams[stream][sid]; !ok {
		return
	}

	if _, ok := h.streams[stream][sid][identifier]; !ok {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.streams[stream][sid], identifier)

	h.log.WithFields(log.Fields{
		"sid":     sid,
		"channel": identifier,
		"stream":  stream,
	}).Debug("Unsubscribed")
}

func (h *Hub) broadcastToStream(stream string, data string) {
	ctx := h.log.WithField("stream", stream)

	ctx.Debugf("Broadcast message: %s", data)

	if _, ok := h.streams[stream]; !ok {
		ctx.Debug("No sessions")
		return
	}

	h.pool.Schedule(func() {
		h.mu.RLock()
		defer h.mu.RUnlock()

		buf := make(map[string](*common.Reply))

		var bdata *common.Reply

		for sid, ids := range h.streams[stream] {
			session, ok := h.sessions[sid]

			if !ok {
				continue
			}

			for id := range ids {
				if msg, ok := buf[id]; ok {
					bdata = msg
				} else {
					bdata = buildMessage(data, id)
					buf[id] = bdata
				}

				session.Send(bdata)
				session.Flush()
			}
		}
	})
}

func (h *Hub) disconnectSessions(identifier string, reconnect bool) {
	ids, ok := h.identifiers[identifier]

	if !ok {
		h.log.Debugf("Can not disconnect sessions: unknown identifier %s", identifier)
		return
	}

	disconnectMessage := newDisconnectMessage(remoteDisconnectReason, reconnect)

	h.pool.Schedule(func() {
		h.mu.RLock()
		defer h.mu.RUnlock()

		for id := range ids {
			if ses, ok := h.sessions[id]; ok {
				ses.Send(disconnectMessage)
				ses.Disconnect("Closed remotely", CloseNormalClosure)
				ses.Flush()
			}
		}
	})
}

func buildMessage(data string, identifier string) *common.Reply {
	var msg interface{}

	// We ignore JSON deserialization failures and consider the message to be a string
	json.Unmarshal([]byte(data), &msg) // nolint:errcheck

	return (&common.Reply{Identifier: identifier, Message: msg})
}
