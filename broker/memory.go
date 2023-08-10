package broker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/anycable/anycable-go/common"
	nanoid "github.com/matoous/go-nanoid"
)

type entry struct {
	timestamp int64
	offset    uint64
	data      string
}

type memstream struct {
	offset   uint64
	deadline int64
	// The lowest available offset in the stream
	low   uint64
	data  []*entry
	ttl   int64
	limit int

	mu sync.RWMutex
}

func (ms *memstream) add(data string) uint64 {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ts := time.Now().Unix()

	ms.offset++

	entry := &entry{
		offset:    ms.offset,
		timestamp: ts,
		data:      data,
	}

	ms.data = append(ms.data, entry)

	if len(ms.data) > ms.limit {
		ms.data = ms.data[1:]
		ms.low = ms.data[0].offset
	}

	if ms.low == 0 {
		ms.low = ms.data[0].offset
	}

	// Update memstream expiration deadline on every added item
	// We keep memstream alive for 10 times longer than ttl (so we can re-use it and its offset)
	ms.deadline = time.Now().Add(time.Duration(ms.ttl*10) * time.Second).Unix()

	return ms.offset
}

func (ms *memstream) expire() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	cutIndex := 0

	now := time.Now().Unix()
	deadline := now - ms.ttl

	for _, entry := range ms.data {
		if entry.timestamp < deadline {
			cutIndex++
			continue
		}

		break
	}

	if cutIndex < 0 {
		return
	}

	ms.data = ms.data[cutIndex:]

	if len(ms.data) > 0 {
		ms.low = ms.data[0].offset
	} else {
		ms.low = 0
	}
}

func (ms *memstream) filterByOffset(offset uint64, callback func(e *entry)) error {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if ms.low > offset {
		return fmt.Errorf("Requested offset couldn't be found: %d, lowest: %d", offset, ms.low)
	}

	if ms.low == 0 {
		return fmt.Errorf("Stream is empty")
	}

	start := (offset - ms.low) + 1

	if start > uint64(len(ms.data)) {
		return fmt.Errorf("Requested offset couldn't be found: %d, latest: %d", offset, ms.data[len(ms.data)-1].offset)
	}

	for _, v := range ms.data[start:] {
		callback(v)
	}

	return nil
}

func (ms *memstream) filterByTime(since int64, callback func(e *entry)) error {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	for _, v := range ms.data {
		if v.timestamp >= since {
			callback(v)
		}
	}

	return nil
}

type sessionEntry struct {
	data []byte
}

type expireSessionEntry struct {
	deadline int64
	sid      string
}

type Memory struct {
	broadcaster    Broadcaster
	config         *Config
	tracker        *StreamsTracker
	epoch          string
	streams        map[string]*memstream
	sessions       map[string]*sessionEntry
	expireSessions []*expireSessionEntry

	streamsMu  sync.RWMutex
	sessionsMu sync.RWMutex
}

var _ Broker = (*Memory)(nil)

func NewMemoryBroker(node Broadcaster, config *Config) *Memory {
	epoch, _ := nanoid.Nanoid(4)

	return &Memory{
		broadcaster: node,
		config:      config,
		tracker:     NewStreamsTracker(),
		streams:     make(map[string]*memstream),
		sessions:    make(map[string]*sessionEntry),
		epoch:       epoch,
	}
}

func (b *Memory) Announce() string {
	return fmt.Sprintf(
		"Using in-memory broker (epoch: %s, history limit: %d, history ttl: %ds, sessions ttl: %ds)",
		b.epoch,
		b.config.HistoryLimit,
		b.config.HistoryTTL,
		b.config.SessionsTTL,
	)
}

func (b *Memory) GetEpoch() string {
	return b.epoch
}

func (b *Memory) SetEpoch(v string) {
	b.epoch = v
}

func (b *Memory) Start() error {
	go b.expireLoop()

	return nil
}

func (b *Memory) Shutdown(ctx context.Context) error {
	return nil
}

func (b *Memory) HandleBroadcast(msg *common.StreamMessage) {
	offset := b.add(msg.Stream, msg.Data)

	msg.Epoch = b.epoch
	msg.Offset = offset

	if b.tracker.Has(msg.Stream) {
		b.broadcaster.Broadcast(msg)
	}
}

func (b *Memory) HandleCommand(msg *common.RemoteCommandMessage) {
	b.broadcaster.BroadcastCommand(msg)
}

// Registring streams (for granular pub/sub)

func (b *Memory) Subscribe(stream string) string {
	isNew := b.tracker.Add(stream)

	if isNew {
		b.broadcaster.Subscribe(stream)
	}

	return stream
}

func (b *Memory) Unsubscribe(stream string) string {
	isLast := b.tracker.Remove(stream)

	if isLast {
		b.broadcaster.Unsubscribe(stream)
	}

	return stream
}

func (b *Memory) HistoryFrom(name string, epoch string, offset uint64) ([]common.StreamMessage, error) {
	if b.epoch != epoch {
		return nil, fmt.Errorf("Unknown epoch: %s, current: %s", epoch, b.epoch)
	}

	stream := b.get(name)

	if stream == nil {
		return nil, errors.New("Stream not found")
	}

	history := []common.StreamMessage{}

	err := stream.filterByOffset(offset, func(entry *entry) {
		history = append(history, common.StreamMessage{
			Stream: name,
			Data:   entry.data,
			Offset: entry.offset,
			Epoch:  b.epoch,
		})
	})

	if err != nil {
		return nil, err
	}

	return history, nil
}

func (b *Memory) HistorySince(name string, ts int64) ([]common.StreamMessage, error) {
	stream := b.get(name)

	if stream == nil {
		return nil, nil
	}

	history := []common.StreamMessage{}

	err := stream.filterByTime(ts, func(entry *entry) {
		history = append(history, common.StreamMessage{
			Stream: name,
			Data:   entry.data,
			Offset: entry.offset,
			Epoch:  b.epoch,
		})
	})

	if err != nil {
		return nil, err
	}

	return history, nil
}

func (b *Memory) CommitSession(sid string, session Cacheable) error {
	b.sessionsMu.Lock()
	defer b.sessionsMu.Unlock()

	cached, err := session.ToCacheEntry()

	if err != nil {
		return err
	}

	b.sessions[sid] = &sessionEntry{data: cached}

	return nil
}

func (b *Memory) RestoreSession(from string) ([]byte, error) {
	b.sessionsMu.RLock()
	defer b.sessionsMu.RUnlock()

	if cached, ok := b.sessions[from]; ok {
		return cached.data, nil
	}

	return nil, nil
}

func (b *Memory) FinishSession(sid string) error {
	b.sessionsMu.Lock()
	defer b.sessionsMu.Unlock()

	if _, ok := b.sessions[sid]; ok {
		b.expireSessions = append(
			b.expireSessions,
			&expireSessionEntry{sid: sid, deadline: time.Now().Unix() + b.config.SessionsTTL},
		)
	}

	return nil
}

func (b *Memory) add(name string, data string) uint64 {
	b.streamsMu.Lock()

	if _, ok := b.streams[name]; !ok {
		b.streams[name] = &memstream{
			data:  []*entry{},
			ttl:   b.config.HistoryTTL,
			limit: b.config.HistoryLimit,
		}
	}

	stream := b.streams[name]

	b.streamsMu.Unlock()

	return stream.add(data)
}

func (b *Memory) get(name string) *memstream {
	b.streamsMu.RLock()
	defer b.streamsMu.RUnlock()

	return b.streams[name]
}

func (b *Memory) expireLoop() {
	for {
		time.Sleep(time.Second)
		b.expire()
	}
}

func (b *Memory) expire() {
	b.streamsMu.Lock()

	toDelete := []string{}

	now := time.Now().Unix()

	for name, stream := range b.streams {
		stream.expire()

		if stream.deadline < now {
			toDelete = append(toDelete, name)
		}
	}

	for _, name := range toDelete {
		delete(b.streams, name)
	}

	b.streamsMu.Unlock()

	b.sessionsMu.Lock()

	i := 0

	for _, expired := range b.expireSessions {
		if expired.deadline < now {
			delete(b.sessions, expired.sid)
			i++
			continue
		}
		break
	}

	b.expireSessions = b.expireSessions[i:]

	b.sessionsMu.Unlock()
}
