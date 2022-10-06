package broker

import (
	"errors"
	"sync"

	"github.com/anycable/anycable-go/common"
)

const (
	SESSION_ID_HEADER = "X-ANYCABLE-RESTORE-SID"
)

// Broadcaster is responsible for fan-out message to the stream clients
type Broadcaster interface {
	Broadcast(msg *common.StreamMessage)
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
// - Keeping client states for recovery.
// - Distributing broadcasts across nodes.
type Broker interface {
	Start() error
	Shutdown() error

	Announce() string

	HandleBroadcast(msg *common.StreamMessage)

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
	// Marks session as finished (for cache expiration)
	FinishSession(sid string) error
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

	return false
}

// LegacyBroker preserves the v1 behaviour while implementing the Broker APIs.
// Thus, we can use it without breaking the older behaviour
type LegacyBroker struct {
	broadcaster Broadcaster
}

func NewLegacyBroker(broadcaster Broadcaster) *LegacyBroker {
	return &LegacyBroker{broadcaster: broadcaster}
}

func (LegacyBroker) Start() error {
	return nil
}

func (LegacyBroker) Shutdown() error {
	return nil
}

func (LegacyBroker) Announce() string {
	return "Using no-op (legacy) broker"
}

func (b *LegacyBroker) HandleBroadcast(msg *common.StreamMessage) {
	b.broadcaster.Broadcast(msg)
}

// Registring streams (for granular pub/sub)

func (LegacyBroker) Subscribe(stream string) string {
	return stream
}

func (LegacyBroker) Unsubscribe(stream string) string {
	return stream
}

func (LegacyBroker) HistoryFrom(stream string, epoch string, offset uint64) ([]common.StreamMessage, error) {
	return nil, errors.New("History not supported")
}

func (LegacyBroker) HistorySince(stream string, ts int64) ([]common.StreamMessage, error) {
	return nil, errors.New("History not supported")
}

func (LegacyBroker) CommitSession(sid string, session Cacheable) error {
	return nil
}

func (LegacyBroker) RestoreSession(from string) ([]byte, error) {
	return nil, nil
}

func (LegacyBroker) FinishSession(sid string) error {
	return nil
}
