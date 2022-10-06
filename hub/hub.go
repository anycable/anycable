package hub

import (
	"sync"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
	"github.com/anycable/anycable-go/utils"
	"github.com/apex/log"
)

type HubSession interface {
	GetID() string
	GetIdentifiers() string
	Send(msg encoders.EncodedMessage)
	DisconnectWithMessage(msg encoders.EncodedMessage, code string)
}

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
	session HubSession
}

// Hub stores all the sessions and the corresponding subscriptions info
type Hub struct {
	// Registered sessions
	sessions map[string]HubSession

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

	// mutex for streams mappings
	streamsMu sync.RWMutex

	// mutex for sessions tracking
	sessionsMu sync.RWMutex
}

// NewHub builds new hub instance
func NewHub(poolSize int) *Hub {
	return &Hub{
		broadcast:       make(chan *common.StreamMessage, 256),
		disconnect:      make(chan *common.RemoteDisconnectMessage, 128),
		register:        make(chan HubRegistration, 2048),
		subscribe:       make(chan HubSubscription, 128),
		sessions:        make(map[string]HubSession),
		identifiers:     make(map[string]map[string]bool),
		streams:         make(map[string]map[string]map[string]bool),
		sessionsStreams: make(map[string]map[string][]string),
		shutdown:        make(chan struct{}),
		log:             log.WithFields(log.Fields{"context": "hub"}),
		pool:            utils.NewGoPool("broadcast", poolSize),
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

		case subinfo := <-h.subscribe:
			switch subinfo.event {
			case "add":
				{
					h.SubscribeSession(subinfo.session, subinfo.stream, subinfo.identifier)
				}
			case "removeAll":
				{
					h.unsubscribeSessionFromChannel(subinfo.session, subinfo.identifier, false)
				}
			default:
				{
					h.UnsubscribeSession(subinfo.session, subinfo.stream, subinfo.identifier)
				}
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
	h.sessionsMu.RLock()
	defer h.sessionsMu.RUnlock()

	return len(h.sessions)
}

// UniqSize returns a number of uniq identifiers
func (h *Hub) UniqSize() int {
	h.sessionsMu.RLock()
	defer h.sessionsMu.RUnlock()

	return len(h.identifiers)
}

// StreamsSize returns a number of uniq streams
func (h *Hub) StreamsSize() int {
	h.streamsMu.RLock()
	defer h.streamsMu.RUnlock()

	return len(h.streams)
}

func (h *Hub) AddSession(session HubSession) {
	h.sessionsMu.Lock()
	defer h.sessionsMu.Unlock()

	uid := session.GetID()
	identifiers := session.GetIdentifiers()

	h.sessions[uid] = session

	if _, ok := h.identifiers[identifiers]; !ok {
		h.identifiers[identifiers] = make(map[string]bool)
	}

	h.identifiers[identifiers][uid] = true

	h.log.WithField("sid", uid).Debugf(
		"Registered with identifiers: %s",
		identifiers,
	)
}

func (h *Hub) RemoveSession(session HubSession) {
	h.sessionsMu.RLock()
	uid := session.GetID()

	if _, ok := h.sessions[uid]; !ok {
		h.sessionsMu.RUnlock()
		h.log.WithField("sid", uid).Warn("Session hasn't been registered")
		return
	}
	h.sessionsMu.RUnlock()

	identifiers := session.GetIdentifiers()
	h.unsubscribeSessionFromAllChannels(uid)

	h.sessionsMu.Lock()

	delete(h.sessions, uid)
	delete(h.identifiers[identifiers], uid)

	if len(h.identifiers[identifiers]) == 0 {
		delete(h.identifiers, identifiers)
	}

	h.sessionsMu.Unlock()

	h.log.WithField("sid", uid).Debug("Unregistered")
}

func (h *Hub) unsubscribeSessionFromAllChannels(sid string) {
	h.streamsMu.Lock()
	defer h.streamsMu.Unlock()

	for channel := range h.sessionsStreams[sid] {
		h.unsubscribeSessionFromChannel(sid, channel, true)
	}

	delete(h.sessionsStreams, sid)
}

func (h *Hub) UnsubscribeSessionFromChannel(sid string, identifier string) {
	h.unsubscribeSessionFromChannel(sid, identifier, false)
}

func (h *Hub) unsubscribeSessionFromChannel(sid string, identifier string, locked bool) {
	if !locked {
		h.streamsMu.Lock()
		defer h.streamsMu.Unlock()
	}

	if _, ok := h.sessionsStreams[sid]; !ok {
		return
	}

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

func (h *Hub) SubscribeSession(sid string, stream string, identifier string) {
	h.streamsMu.Lock()
	defer h.streamsMu.Unlock()

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

func (h *Hub) UnsubscribeSession(sid string, stream string, identifier string) {
	h.streamsMu.RLock()
	if _, ok := h.streams[stream]; !ok {
		h.streamsMu.RUnlock()
		return
	}

	if _, ok := h.streams[stream][sid]; !ok {
		h.streamsMu.RUnlock()
		return
	}

	if _, ok := h.streams[stream][sid][identifier]; !ok {
		h.streamsMu.RUnlock()
		return
	}

	h.streamsMu.RUnlock()

	h.streamsMu.Lock()
	defer h.streamsMu.Unlock()

	delete(h.streams[stream][sid], identifier)

	h.log.WithFields(log.Fields{
		"sid":     sid,
		"channel": identifier,
		"stream":  stream,
	}).Debug("Unsubscribed")
}

func (h *Hub) broadcastToStream(streamMsg *common.StreamMessage) {
	stream := streamMsg.Stream

	ctx := h.log.WithField("stream", stream)

	ctx.Debugf("Broadcast message: %v", streamMsg)

	h.streamsMu.RLock()
	if _, ok := h.streams[stream]; !ok {
		ctx.Debug("No sessions")
		h.streamsMu.RUnlock()
		return
	}
	h.streamsMu.RUnlock()

	h.pool.Schedule(func() {
		buf := make(map[string](encoders.EncodedMessage))

		var bdata encoders.EncodedMessage

		h.streamsMu.RLock()
		streamSessions := streamSessionsSnapshot(h.streams[stream])
		h.streamsMu.RUnlock()

		for sid, ids := range streamSessions {
			h.sessionsMu.RLock()
			session, ok := h.sessions[sid]
			h.sessionsMu.RUnlock()

			if !ok {
				continue
			}

			for _, id := range ids {
				if msg, ok := buf[id]; ok {
					bdata = msg
				} else {
					bdata = buildMessage(streamMsg, id)
					buf[id] = bdata
				}

				session.Send(bdata)
			}
		}
	})
}

func (h *Hub) disconnectSessions(identifier string, reconnect bool) {
	h.sessionsMu.RLock()
	ids, ok := h.identifiers[identifier]
	h.sessionsMu.RUnlock()

	if !ok {
		h.log.Debugf("Can not disconnect sessions: unknown identifier %s", identifier)
		return
	}

	msg := common.NewDisconnectMessage(common.REMOTE_DISCONNECT_REASON, reconnect)

	h.pool.Schedule(func() {
		h.sessionsMu.RLock()
		defer h.sessionsMu.RUnlock()

		for id := range ids {
			if ses, ok := h.sessions[id]; ok {
				ses.DisconnectWithMessage(msg, common.REMOTE_DISCONNECT_REASON)
			}
		}
	})
}

func (h *Hub) FindByIdentifier(id string) HubSession {
	h.sessionsMu.RLock()
	defer h.sessionsMu.RUnlock()

	ids, ok := h.identifiers[id]

	if !ok {
		return nil
	}

	for id := range ids {
		if ses, ok := h.sessions[id]; ok {
			return ses
		}
	}

	return nil
}

func (h *Hub) DisconnectSesssions(msg encoders.EncodedMessage, code string) {
	h.sessionsMu.RLock()
	for _, session := range h.sessions {
		session.DisconnectWithMessage(msg, code)
	}
	h.sessionsMu.RUnlock()
}

func buildMessage(msg *common.StreamMessage, identifier string) encoders.EncodedMessage {
	return encoders.NewCachedEncodedMessage(msg.ToReplyFor(identifier))
}

func streamSessionsSnapshot(src map[string]map[string]bool) map[string][]string {
	dest := make(map[string][]string)

	for k, v := range src {
		dest[k] = make([]string, len(v))

		i := 0

		for id := range v {
			dest[k][i] = id
			i++
		}
	}

	return dest
}
