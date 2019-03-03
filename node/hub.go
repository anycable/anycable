package node

import (
	"encoding/json"
	"sync"

	"github.com/apex/log"
)

// SubscriptionInfo contains information about session-channel(-stream) subscription
type SubscriptionInfo struct {
	session    string
	stream     string
	identifier string
}

// Reply represents outgoing client message
type Reply struct {
	Type       string      `json:"type,omitempty"`
	Identifier string      `json:"identifier"`
	Message    interface{} `json:"message"`
}

func (r *Reply) toJSON() []byte {
	jsonStr, err := json.Marshal(&r)
	if err != nil {
		panic("Failed to build JSON")
	}
	return jsonStr
}

// Hub stores all the sessions and the corresponding subscriptions info
type Hub struct {
	// Registered sessions
	sessions map[string]*Session

	// Identifiers to session
	identifiers map[string]map[string]bool

	// Maps streams to sessions
	streams map[string]map[string]string

	// Maps sessions to identifiers to streams
	sessionsStreams map[string]map[string][]string

	// Messages for specified stream
	broadcast chan *StreamMessage

	// Register requests from the sessions
	register chan *Session

	// Unregister requests from sessions
	unregister chan *Session

	// Subscribe requests to streams
	subscribe chan *SubscriptionInfo

	// Unsubscribe requests from streams
	unsubscribe chan *SubscriptionInfo

	// Control channel to shutdown hub
	shutdown chan struct{}

	// Synchronization group to wait for gracefully disconnect of all sessions
	done sync.WaitGroup

	// Log context
	log *log.Entry
}

// NewHub builds new hub instance
func NewHub() *Hub {
	return &Hub{
		broadcast:       make(chan *StreamMessage, 256),
		register:        make(chan *Session, 128),
		unregister:      make(chan *Session, 2048),
		subscribe:       make(chan *SubscriptionInfo, 128),
		unsubscribe:     make(chan *SubscriptionInfo, 128),
		sessions:        make(map[string]*Session),
		identifiers:     make(map[string]map[string]bool),
		streams:         make(map[string]map[string]string),
		sessionsStreams: make(map[string]map[string][]string),
		shutdown:        make(chan struct{}),
		log:             log.WithFields(log.Fields{"context": "hub"}),
	}
}

// Run makes hub active
func (h *Hub) Run() {
	h.done.Add(1)
	for {
		select {
		case s := <-h.register:
			h.addSession(s)

		case s := <-h.unregister:
			h.removeSession(s)

		case subinfo := <-h.subscribe:
			h.subscribeSession(subinfo.session, subinfo.stream, subinfo.identifier)

		case subinfo := <-h.unsubscribe:
			h.unsubscribeSessionFromChannel(subinfo.session, subinfo.identifier)

		case message := <-h.broadcast:
			h.broadcastToStream(message.Stream, message.Data)

		case <-h.shutdown:
			h.done.Done()
			return
		}
	}
}

// Shutdown sends shutdown command to hub
func (h *Hub) Shutdown() {
	h.shutdown <- struct{}{}

	// Wait for stop listening channels
	h.done.Wait()
}

// Size returns a number of active sessions
func (h *Hub) Size() int {
	return len(h.sessions)
}

// UniqSize returns a number of uniq identifiers
func (h *Hub) UniqSize() int {
	return len(h.identifiers)
}

// StreamsSize returns a number of uniq streams
func (h *Hub) StreamsSize() int {
	return len(h.streams)
}

func (h *Hub) addSession(session *Session) {
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

	for _, stream := range h.sessionsStreams[sid][identifier] {
		delete(h.streams[stream], sid)

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
	if _, ok := h.streams[stream]; !ok {
		h.streams[stream] = make(map[string]string)
	}

	h.streams[stream][sid] = identifier

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

func (h *Hub) broadcastToStream(stream string, data string) {
	ctx := h.log.WithField("stream", stream)

	ctx.Debugf("Broadcast message: %s", data)

	if _, ok := h.streams[stream]; !ok {
		ctx.Debug("No sessions")
		return
	}

	buf := make(map[string][]byte)

	for sid, id := range h.streams[stream] {
		session, ok := h.sessions[sid]

		if !ok {
			continue
		}

		var bdata []byte

		if msg, ok := buf[id]; ok {
			bdata = msg
		} else {
			bdata = buildMessage(data, id)
			buf[id] = bdata
		}

		session.Send(bdata)
	}
}

func buildMessage(data string, identifier string) []byte {
	var msg interface{}

	json.Unmarshal([]byte(data), &msg)

	return (&Reply{Identifier: identifier, Message: msg}).toJSON()
}
