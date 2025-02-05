package broker

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/utils"
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

	ms.appendEntry(entry)

	return ms.offset
}

func (ms *memstream) insert(data string, offset uint64, t time.Time) (uint64, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if t == (time.Time{}) {
		t = time.Now()
	}

	ts := t.Unix()

	if ms.offset >= offset {
		return 0, fmt.Errorf("offset %d is already taken", offset)
	}

	ms.offset = offset

	entry := &entry{
		offset:    offset,
		timestamp: ts,
		data:      data,
	}

	ms.appendEntry(entry)

	return ms.offset, nil
}

func (ms *memstream) appendEntry(entry *entry) {
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
		return fmt.Errorf("requested offset couldn't be found: %d, lowest: %d", offset, ms.low)
	}

	if ms.low == 0 {
		return fmt.Errorf("stream is empty")
	}

	start := (offset - ms.low) + 1

	if start > uint64(len(ms.data)) {
		return fmt.Errorf("requested offset couldn't be found: %d, latest: %d", offset, ms.data[len(ms.data)-1].offset)
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
	data       []byte
	expiration *utils.PriorityQueueItem[string, int64]
}

type presenceSessionEntry struct {
	// stream -> pid
	streams  map[string]string
	deadline int64
}

type presenceEntry struct {
	info     interface{}
	id       string
	sessions []string
}

func (pe *presenceEntry) remove(sid string) bool {
	i := -1

	for idx, s := range pe.sessions {
		if s == sid {
			i = idx
			break
		}
	}

	if i == -1 {
		return false
	}

	pe.sessions = append(pe.sessions[:i], pe.sessions[i+1:]...)

	return len(pe.sessions) == 0
}

func (pe *presenceEntry) add(sid string, info interface{}) {
	if !slices.Contains(pe.sessions, sid) {
		pe.sessions = append(pe.sessions, sid)
	}

	pe.info = info
}

type presenceState struct {
	streams  map[string]map[string]*presenceEntry
	sessions map[string]*presenceSessionEntry

	mu sync.RWMutex
}

func newPresenceState() *presenceState {
	return &presenceState{
		streams:  make(map[string]map[string]*presenceEntry),
		sessions: make(map[string]*presenceSessionEntry),
	}
}

type Memory struct {
	broadcaster    Broadcaster
	config         *Config
	tracker        *StreamsTracker
	epoch          string
	streams        map[string]*memstream
	sessions       map[string]*sessionEntry
	expireSessions *utils.PriorityQueue[string, int64]

	presence *presenceState

	streamsMu  sync.RWMutex
	sessionsMu sync.RWMutex
	epochMu    sync.RWMutex
}

var _ Broker = (*Memory)(nil)

func NewMemoryBroker(node Broadcaster, config *Config) *Memory {
	epoch, _ := nanoid.Nanoid(4)

	return &Memory{
		broadcaster:    node,
		config:         config,
		tracker:        NewStreamsTracker(),
		streams:        make(map[string]*memstream),
		sessions:       make(map[string]*sessionEntry),
		expireSessions: utils.NewPriorityQueue[string, int64](),
		presence:       newPresenceState(),
		epoch:          epoch,
	}
}

func (b *Memory) Announce() string {
	return fmt.Sprintf(
		"Using in-memory broker (epoch: %s, history limit: %d, history ttl: %ds, sessions ttl: %ds, presence ttl: %ds)",
		b.GetEpoch(),
		b.config.HistoryLimit,
		b.config.HistoryTTL,
		b.config.SessionsTTL,
		b.config.PresenceTTL,
	)
}

func (b *Memory) GetEpoch() string {
	b.epochMu.RLock()
	defer b.epochMu.RUnlock()

	return b.epoch
}

func (b *Memory) SetEpoch(v string) {
	b.epochMu.Lock()
	defer b.epochMu.Unlock()

	b.epoch = v
}

func (b *Memory) Start(done chan (error)) error {
	go b.expireLoop()

	return nil
}

func (b *Memory) Shutdown(ctx context.Context) error {
	return nil
}

func (b *Memory) HandleBroadcast(msg *common.StreamMessage) {
	if msg.Meta != nil && msg.Meta.Transient {
		b.broadcaster.Broadcast(msg)
		return
	}

	offset := b.add(msg.Stream, msg.Data)

	msg.Epoch = b.GetEpoch()
	msg.Offset = offset

	b.broadcaster.Broadcast(msg)
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
	bepoch := b.GetEpoch()

	if bepoch != epoch {
		return nil, fmt.Errorf("unknown epoch: %s, current: %s", epoch, bepoch)
	}

	stream := b.get(name)

	if stream == nil {
		return nil, errors.New("stream not found")
	}

	history := []common.StreamMessage{}

	err := stream.filterByOffset(offset, func(entry *entry) {
		history = append(history, common.StreamMessage{
			Stream: name,
			Data:   entry.data,
			Offset: entry.offset,
			Epoch:  bepoch,
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

	bepoch := b.GetEpoch()
	history := []common.StreamMessage{}

	err := stream.filterByTime(ts, func(entry *entry) {
		history = append(history, common.StreamMessage{
			Stream: name,
			Data:   entry.data,
			Offset: entry.offset,
			Epoch:  bepoch,
		})
	})

	if err != nil {
		return nil, err
	}

	return history, nil
}

func (b *Memory) Store(name string, data []byte, offset uint64, ts time.Time) (uint64, error) {
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

	return stream.insert(string(data), offset, ts)
}

func (b *Memory) CommitSession(sid string, session Cacheable) error {
	b.sessionsMu.Lock()
	defer b.sessionsMu.Unlock()

	cached, err := session.ToCacheEntry()

	if err != nil {
		return err
	}

	deadline := time.Now().UnixMilli() + (b.config.SessionsTTL * 1000)
	var expiration *utils.PriorityQueueItem[string, int64]

	if entry, ok := b.sessions[sid]; ok {
		expiration = entry.expiration
		b.expireSessions.Update(expiration, deadline)
	} else {
		expiration = b.expireSessions.PushItem(sid, deadline)
	}

	b.sessions[sid] = &sessionEntry{data: cached, expiration: expiration}

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

func (b *Memory) TouchSession(sid string) error {
	b.sessionsMu.Lock()
	if entry, ok := b.sessions[sid]; ok {
		b.expireSessions.Update(entry.expiration, time.Now().UnixMilli()+(b.config.SessionsTTL*1000))
	}
	b.sessionsMu.Unlock()

	return nil
}

func (b *Memory) TouchPresence(sid string) error {
	b.presence.mu.Lock()

	if sp, ok := b.presence.sessions[sid]; ok {
		sp.deadline = time.Now().UnixMilli() + (b.config.PresenceTTL * 1000)
	}

	b.presence.mu.Unlock()

	return nil
}

func (b *Memory) PresenceAdd(stream string, sid string, pid string, info interface{}) (*common.PresenceEvent, error) {
	b.presence.mu.Lock()
	defer b.presence.mu.Unlock()

	if _, ok := b.presence.streams[stream]; !ok {
		b.presence.streams[stream] = make(map[string]*presenceEntry)
	}

	if _, ok := b.presence.sessions[sid]; !ok {
		b.presence.sessions[sid] = &presenceSessionEntry{
			streams:  make(map[string]string),
			deadline: time.Now().UnixMilli() + (b.config.PresenceTTL * 1000),
		}
	}

	if oldPid, ok := b.presence.sessions[sid].streams[stream]; ok && oldPid != pid {
		return nil, errors.New("presence ID mismatch")
	}

	b.presence.sessions[sid].streams[stream] = pid

	streamPresence := b.presence.streams[stream]

	newPresence := false

	if _, ok := streamPresence[pid]; !ok {
		newPresence = true
		streamPresence[pid] = &presenceEntry{
			info:     info,
			id:       pid,
			sessions: []string{},
		}
	}

	streamSessionPresence := streamPresence[pid]

	streamSessionPresence.add(sid, info)

	if newPresence {
		return &common.PresenceEvent{
			Type: common.PresenceJoinType,
			ID:   pid,
			Info: info,
		}, nil
	}

	return nil, nil
}

func (b *Memory) PresenceRemove(stream string, sid string) (*common.PresenceEvent, error) {
	b.presence.mu.Lock()
	defer b.presence.mu.Unlock()

	if _, ok := b.presence.streams[stream]; !ok {
		return nil, errors.New("stream not found")
	}

	var pid string

	if ses, ok := b.presence.sessions[sid]; ok {
		if id, ok := ses.streams[stream]; !ok {
			return nil, errors.New("presence info not found")
		} else {
			pid = id
		}

		delete(ses.streams, stream)

		if len(ses.streams) == 0 {
			delete(b.presence.sessions, sid)
		}
	}

	streamPresence := b.presence.streams[stream]

	if _, ok := streamPresence[pid]; !ok {
		return nil, errors.New("presence record not found")
	}

	streamSessionPresence := streamPresence[pid]

	empty := streamSessionPresence.remove(sid)

	if empty {
		delete(streamPresence, pid)
	}

	if len(streamPresence) == 0 {
		delete(b.presence.streams, stream)
	}

	if empty {
		return &common.PresenceEvent{
			Type: common.PresenceLeaveType,
			ID:   pid,
		}, nil
	}

	return nil, nil
}

func (b *Memory) PresenceInfo(stream string, opts ...PresenceInfoOption) (*common.PresenceInfo, error) {
	options := NewPresenceInfoOptions()
	for _, opt := range opts {
		opt(options)
	}

	b.presence.mu.RLock()
	defer b.presence.mu.RUnlock()

	info := common.NewPresenceInfo()

	if _, ok := b.presence.streams[stream]; !ok {
		return info, nil
	}

	streamPresence := b.presence.streams[stream]

	info.Total = len(streamPresence)

	if options.ReturnRecords {
		info.Records = make([]*common.PresenceEvent, 0, len(streamPresence))

		for _, entry := range streamPresence {
			info.Records = append(info.Records, &common.PresenceEvent{
				Info: entry.info,
				ID:   entry.id,
			})
		}
	}

	return info, nil
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

	// sessions expiration
	b.expireSessionsCache()

	// presence expiration
	b.expirePresence()
}

func (b *Memory) expireSessionsCache() {
	b.sessionsMu.Lock()

	now := time.Now().UnixMilli()

	for {
		item := b.expireSessions.Peek()

		if item == nil || item.Priority() > now {
			break
		}

		b.expireSessions.PopItem()

		delete(b.sessions, item.Value())
	}

	b.sessionsMu.Unlock()
}

func (b *Memory) expirePresence() {
	b.presence.mu.Lock()

	now := time.Now().UnixMilli()
	toDelete := []string{}

	for sid, sp := range b.presence.sessions {
		if sp.deadline > 0 && sp.deadline < now {
			toDelete = append(toDelete, sid)
		}
	}

	leaveMessages := []common.StreamMessage{}

	for _, sid := range toDelete {
		entry := b.presence.sessions[sid]

		for stream, pid := range entry.streams {
			if _, ok := b.presence.streams[stream]; !ok {
				continue
			}

			if _, ok := b.presence.streams[stream][pid]; !ok {
				continue
			}

			streamSessionPresence := b.presence.streams[stream][pid]

			empty := streamSessionPresence.remove(sid)

			if empty {
				delete(b.presence.streams[stream], pid)

				msg := &common.PresenceEvent{Type: common.PresenceLeaveType, ID: pid}

				leaveMessages = append(leaveMessages, common.StreamMessage{
					Stream: stream,
					Data:   string(utils.ToJSON(msg)),
					Meta: &common.StreamMessageMetadata{
						BroadcastType: common.PresenceType,
						Transient:     true,
					},
				})

				if len(b.presence.streams[stream]) == 0 {
					delete(b.presence.streams, stream)
				}
			}
		}

		delete(b.presence.sessions, sid)
	}

	b.presence.mu.Unlock()

	if b.broadcaster != nil {
		// TODO: batch broadcast?
		// FIXME: move broadcasts out of broker
		for _, msg := range leaveMessages {
			b.broadcaster.Broadcast(&msg)
		}
	}
}
