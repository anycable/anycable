package hub

import (
	"context"
	"log/slog"
	"sync"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/encoders"
)

// Gate plays the role of a shard for the hub.
// It keeps subscriptions for some streams (a particular shard) and is used
// to broadcast messages to all subscribers of these streams.
type Gate struct {
	// Maps streams to sessions with identifiers
	// stream -> session -> identifier -> true
	streams map[string]map[HubSession]map[string]bool

	// Maps sessions to identifiers to streams
	// session -> identifier -> [stream]
	sessionsStreams map[HubSession]map[string][]string

	// This channel is used as a broadcast queue
	sender chan *common.StreamMessage

	mu  sync.RWMutex
	log *slog.Logger
}

// NewGate creates a new gate.
func NewGate(ctx context.Context, l *slog.Logger) *Gate {
	g := Gate{
		streams:         make(map[string]map[HubSession]map[string]bool),
		sessionsStreams: make(map[HubSession]map[string][]string),
		// Use a buffered channel to avoid blocking
		sender: make(chan *common.StreamMessage, 256),
		log:    l,
	}

	go g.broadcastLoop(ctx)

	return &g
}

// Broadcast sends a message to all subscribers of the stream.
func (g *Gate) Broadcast(streamMsg *common.StreamMessage) {
	stream := streamMsg.Stream

	ctx := g.log.With("stream", stream)

	ctx.Debug("broadcast message", "stream", streamMsg, "data", streamMsg.Data, "offset", streamMsg.Offset, "epoch", streamMsg.Epoch, "meta", streamMsg.Meta)

	g.mu.RLock()
	if _, ok := g.streams[stream]; !ok {
		ctx.Debug("no sessions")
		g.mu.RUnlock()
		return
	}
	g.mu.RUnlock()

	g.sender <- streamMsg
}

// Subscribe adds a session to the stream.
func (g *Gate) Subscribe(session HubSession, stream string, identifier string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.streams[stream]; !ok {
		g.streams[stream] = make(map[HubSession]map[string]bool)
	}

	if _, ok := g.streams[stream][session]; !ok {
		g.streams[stream][session] = make(map[string]bool)
	}

	g.streams[stream][session][identifier] = true
}

// Unsubscribe removes a session from the stream.
func (g *Gate) Unsubscribe(session HubSession, stream string, identifier string) {
	g.mu.RLock()

	if _, ok := g.streams[stream]; !ok {
		g.mu.RUnlock()
		return
	}

	if _, ok := g.streams[stream][session]; !ok {
		g.mu.RUnlock()
		return
	}

	if _, ok := g.streams[stream][session][identifier]; !ok {
		g.mu.RUnlock()
		return
	}

	g.mu.RUnlock()

	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.streams[stream][session], identifier)

	if len(g.streams[stream][session]) == 0 {
		delete(g.streams[stream], session)

		if len(g.streams[stream]) == 0 {
			delete(g.streams, stream)
		}
	}
}

// Size returns a number of uniq streams
func (g *Gate) Size() int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return len(g.streams)
}

func (g *Gate) broadcastLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-g.sender:
			g.performBroadcast(msg)
		}
	}
}

func (g *Gate) performBroadcast(streamMsg *common.StreamMessage) {
	stream := streamMsg.Stream

	buf := make(map[string](encoders.EncodedMessage))

	var bdata encoders.EncodedMessage

	g.mu.RLock()
	streamSessions := streamSessionsSnapshot(g.streams[stream])
	g.mu.RUnlock()

	for session, ids := range streamSessions {
		if streamMsg.Meta != nil && streamMsg.Meta.ExcludeSocket == session.GetID() {
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
}

func buildMessage(msg *common.StreamMessage, identifier string) encoders.EncodedMessage {
	return encoders.NewCachedEncodedMessage(msg.ToReplyFor(identifier))
}

func streamSessionsSnapshot[T comparable](src map[T]map[string]bool) map[T][]string {
	dest := make(map[T][]string)

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
