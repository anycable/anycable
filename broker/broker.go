package broker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/anycable/anycable-go/common"
)

// Broadcaster is responsible for fanning-out messages to the stream clients
// and other nodes
type Broadcaster interface {
	Broadcast(msg *common.StreamMessage)
	BroadcastCommand(msg *common.RemoteCommandMessage)
	Subscribe(stream string)
	Unsubscribe(stream string)
}

// Cacheable is an interface which a session object must implement
// to be stored in cache.
// We use interface and not require a string cache entry to be passed to avoid
// unnecessary dumping when broker doesn't support storing sessions.
type Cacheable interface {
	ToCacheEntry() ([]byte, error)
}

// Broker is responsible for:
// - Managing streams history.
// - Managing presence information.
// - Keeping client states for recovery.
// - Distributing broadcasts across nodes.
//
//go:generate mockery --name Broker --output "../mocks" --outpkg mocks
type Broker interface {
	Start(done chan (error)) error
	Shutdown(ctx context.Context) error

	Announce() string

	HandleBroadcast(msg *common.StreamMessage)
	HandleCommand(msg *common.RemoteCommandMessage)

	// Registers the stream and returns its (short) unique identifier
	Subscribe(stream string) string
	// (Maybe) unregisters the stream and return its unique identifier
	Unsubscribe(stream string) string
	// Retrieves stream messages from history from the specified offset within the specified epoch
	HistoryFrom(stream string, epoch string, offset uint64) ([]common.StreamMessage, error)
	// Retrieves stream messages from history from the specified timestamp
	HistorySince(stream string, ts int64) ([]common.StreamMessage, error)

	// Saves session's state in cache
	CommitSession(sid string, session Cacheable) error
	// Fetches session's state from cache (by session id)
	RestoreSession(from string) ([]byte, error)
	// Prevent session cache from expiration
	TouchSession(sid string) error

	// Adds a new presence record for the stream. Returns true if that's the first
	// presence record for the presence ID (pid, a unique user presence identifier).
	PresenceAdd(stream string, sid string, pid string, info interface{}) (*common.PresenceEvent, error)

	// Removes a presence record for the stream. Returns true if that was the last
	// record for the presence ID (pid).
	PresenceRemove(stream string, sid string) (*common.PresenceEvent, error)

	// Retrieves presence information for the stream (counts, records, etc. depending on the options)
	PresenceInfo(stream string, opts ...PresenceInfoOption) (*common.PresenceInfo, error)

	// Marks presence record as still alive
	TouchPresence(sid string) error
}

// LocalBroker is a single-node broker that can used to store streams data locally
type LocalBroker interface {
	Start(done chan (error)) error
	Shutdown(ctx context.Context) error
	SetEpoch(epoch string)
	GetEpoch() string
	HistoryFrom(stream string, epoch string, offset uint64) ([]common.StreamMessage, error)
	HistorySince(stream string, ts int64) ([]common.StreamMessage, error)
	Store(stream string, msg []byte, seq uint64, ts time.Time) (uint64, error)
}

type StreamsTracker struct {
	store map[string]uint64

	mu sync.Mutex
}

func NewStreamsTracker() *StreamsTracker {
	return &StreamsTracker{store: make(map[string]uint64)}
}

func (s *StreamsTracker) Add(name string) (isNew bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.store[name]; !ok {
		s.store[name] = 1
		return true
	}

	s.store[name]++
	return false
}

func (s *StreamsTracker) Has(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.store[name]

	return ok
}

func (s *StreamsTracker) Remove(name string) (isLast bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.store[name]; !ok {
		return false
	}

	if s.store[name] == 1 {
		delete(s.store, name)
		return true
	}

	s.store[name]--
	return false
}

// LegacyBroker preserves the v1 behaviour while implementing the Broker APIs.
// Thus, we can use it without breaking the older behaviour
type LegacyBroker struct {
	broadcaster Broadcaster
	tracker     *StreamsTracker
}

var _ Broker = (*LegacyBroker)(nil)

func NewLegacyBroker(broadcaster Broadcaster) *LegacyBroker {
	return &LegacyBroker{
		broadcaster: broadcaster,
		tracker:     NewStreamsTracker(),
	}
}

func (LegacyBroker) Start(done chan (error)) error {
	return nil
}

func (LegacyBroker) Shutdown(ctx context.Context) error {
	return nil
}

func (LegacyBroker) Announce() string {
	return "Using no-op (legacy) broker"
}

func (b *LegacyBroker) HandleBroadcast(msg *common.StreamMessage) {
	if b.tracker.Has(msg.Stream) {
		b.broadcaster.Broadcast(msg)
	}
}

func (b *LegacyBroker) HandleCommand(msg *common.RemoteCommandMessage) {
	b.broadcaster.BroadcastCommand(msg)
}

// Registring streams (for granular pub/sub)
func (b *LegacyBroker) Subscribe(stream string) string {
	isNew := b.tracker.Add(stream)

	if isNew {
		b.broadcaster.Subscribe(stream)
	}

	return stream
}

func (b *LegacyBroker) Unsubscribe(stream string) string {
	isLast := b.tracker.Remove(stream)

	if isLast {
		b.broadcaster.Unsubscribe(stream)
	}

	return stream
}

func (LegacyBroker) HistoryFrom(stream string, epoch string, offset uint64) ([]common.StreamMessage, error) {
	return nil, errors.New("history not supported")
}

func (LegacyBroker) HistorySince(stream string, ts int64) ([]common.StreamMessage, error) {
	return nil, errors.New("history not supported")
}

func (LegacyBroker) CommitSession(sid string, session Cacheable) error {
	return nil
}

func (LegacyBroker) RestoreSession(from string) ([]byte, error) {
	return nil, nil
}

func (LegacyBroker) TouchSession(sid string) error {
	return nil
}

func (LegacyBroker) PresenceAdd(stream string, sid string, pid string, info interface{}) (*common.PresenceEvent, error) {
	return nil, errors.New("presence not supported")
}

func (LegacyBroker) PresenceRemove(stream string, sid string) (*common.PresenceEvent, error) {
	return nil, errors.New("presence not supported")
}

func (LegacyBroker) PresenceInfo(stream string, opts ...PresenceInfoOption) (*common.PresenceInfo, error) {
	return nil, errors.New("presence not supported")
}

func (LegacyBroker) TouchPresence(sid string) error {
	return nil
}
