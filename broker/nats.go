package broker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/anycable/anycable-go/common"
	natsconfig "github.com/anycable/anycable-go/nats"
	"github.com/anycable/anycable-go/utils"
	"github.com/joomcode/errorx"
	nanoid "github.com/matoous/go-nanoid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type NATS struct {
	broadcaster Broadcaster
	conf        *Config
	tracker     *StreamsTracker

	nconf *natsconfig.NATSConfig
	conn  *nats.Conn

	js      jetstream.JetStream
	kv      jetstream.KeyValue
	epochKV jetstream.KeyValue

	jstreams   *lru[string]
	jconsumers *lru[jetstream.Consumer]
	streamSync *streamsSynchronizer

	// Local broker is used to keep a copy of stream messages
	local LocalBroker

	clientMu sync.RWMutex
	epochMu  sync.RWMutex

	epoch string

	shutdownCtx context.Context
	shutdownFn  func()

	readyCtx         context.Context
	broadcastBacklog []*common.StreamMessage
	backlogMu        sync.Mutex

	log *slog.Logger
}

const (
	kvBucket       = "_anycable_"
	epochBucket    = "_anycable_epoch_"
	epochKey       = "_epoch_"
	sessionsPrefix = ""
	streamPrefix   = "_ac_"

	jetstreamReadyTimeout = 1 * time.Second
)

var _ Broker = (*NATS)(nil)

type NATSOption func(*NATS)

func WithNATSLocalBroker(b LocalBroker) NATSOption {
	return func(n *NATS) {
		n.local = b
	}
}

func NewNATSBroker(broadcaster Broadcaster, c *Config, nc *natsconfig.NATSConfig, l *slog.Logger, opts ...NATSOption) *NATS {
	shutdownCtx, shutdownFn := context.WithCancel(context.Background())

	n := NATS{
		broadcaster:      broadcaster,
		conf:             c,
		nconf:            nc,
		shutdownCtx:      shutdownCtx,
		shutdownFn:       shutdownFn,
		tracker:          NewStreamsTracker(),
		broadcastBacklog: []*common.StreamMessage{},
		streamSync:       newStreamsSynchronizer(),
		jstreams:         newLRU[string](time.Duration(c.HistoryTTL * int64(time.Second))),
		jconsumers:       newLRU[jetstream.Consumer](time.Duration(c.HistoryTTL * int64(time.Second))),
		log:              l.With("context", "broker").With("provider", "nats"),
	}

	for _, opt := range opts {
		opt(&n)
	}

	if n.local == nil {
		n.local = NewMemoryBroker(nil, nil, c)
	}

	return &n
}

// Write Broker implementtaion here
func (n *NATS) Start(done chan (error)) error {
	n.clientMu.Lock()
	defer n.clientMu.Unlock()

	connectOptions := []nats.Option{
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(n.nconf.MaxReconnectAttempts),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				n.log.Warn("connection failed", "error", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			n.log.Info("connection restored", "url", nc.ConnectedUrl())
		}),
	}

	if n.nconf.DontRandomizeServers {
		connectOptions = append(connectOptions, nats.DontRandomize())
	}

	nc, err := nats.Connect(n.nconf.Servers, connectOptions...)

	if err != nil {
		return err
	}

	n.conn = nc

	readyCtx, readyFn := context.WithCancelCause(context.Background())

	n.readyCtx = readyCtx

	// Initialize JetStream asynchronously, because we may need to wait for JetStream cluster to be ready
	go func() {
		err := n.initJetStreamWithRetry()
		readyFn(err)
		if err != nil && done != nil {
			done <- err
		}

		if err != nil {
			n.backlogFlush()
		}
	}()

	return nil
}

func (n *NATS) Ready(timeout ...time.Duration) error {
	var err error

	if len(timeout) == 0 {
		<-n.readyCtx.Done()
	} else {
		timer := time.After(timeout[0])

		select {
		case <-n.readyCtx.Done():
		case <-timer:
			err = fmt.Errorf("timeout waiting for JetStream to be ready")
		}
	}

	if err != nil {
		return err
	}

	cause := context.Cause(n.readyCtx)

	if cause == context.Canceled {
		return nil
	} else {
		return cause
	}
}

func (n *NATS) initJetStreamWithRetry() error {
	attempt := 0

	for {
		err := n.initJetStream()

		if err == nil {
			return nil
		}

		// delay with exponentional backoff, min 1s, max 60s
		delay := utils.NextRetry(attempt)
		attempt++

		if attempt > 5 {
			return errorx.Decorate(err, "JetStream is unavailable")
		}

		n.log.Warn("JetStream initialization failed", "error", err)

		n.log.Info(fmt.Sprintf("next JetStream initialization attempt in %s", delay))
		time.Sleep(delay)

		n.log.Info("re-initializing JetStream...")
	}
}

func (n *NATS) initJetStream() error {
	n.clientMu.Lock()
	defer n.clientMu.Unlock()

	nc := n.conn
	js, err := jetstream.New(nc)

	if err != nil {
		return errorx.Decorate(err, "failed to connect to JetStream")
	}

	n.js = js

	kv, err := n.fetchBucketWithTTL(kvBucket, time.Duration(n.conf.SessionsTTL*int64(time.Second)))

	if err != nil {
		return errorx.Decorate(err, "failed to connect to JetStream KV")
	}

	n.kv = kv

	epoch, err := n.calculateEpoch()

	if err != nil {
		return errorx.Decorate(err, "failed to calculate epoch")
	}

	n.writeEpoch(epoch)
	err = n.local.Start(nil)

	if err != nil {
		return errorx.Decorate(err, "failed to start internal memory broker")
	}

	err = n.watchEpoch(n.shutdownCtx)

	if err != nil {
		n.log.Warn("failed to set up epoch watcher", "error", err)
	}

	n.log.Info("NATS broker is ready", "epoch", epoch)
	return nil
}

func (n *NATS) Shutdown(ctx context.Context) error {
	n.clientMu.Lock()
	defer n.clientMu.Unlock()

	n.shutdownFn()

	if n.conn != nil {
		n.conn.Close()
		n.conn = nil
	}

	if n.local != nil {
		n.local.Shutdown(ctx) // nolint:errcheck
	}

	return nil
}

func (n *NATS) Announce() string {
	brokerParams := fmt.Sprintf("(history limit: %d, history ttl: %ds, sessions ttl: %ds)", n.conf.HistoryLimit, n.conf.HistoryTTL, n.conf.SessionsTTL)

	return fmt.Sprintf("Using NATS broker: %s %s", n.nconf.Servers, brokerParams)
}

func (n *NATS) Epoch() string {
	n.epochMu.RLock()
	defer n.epochMu.RUnlock()

	return n.epoch
}

func (n *NATS) SetEpoch(epoch string) error {
	n.clientMu.RLock()
	defer n.clientMu.RUnlock()

	bucket, err := n.js.KeyValue(context.Background(), epochBucket)

	if err != nil {
		return err
	}

	_, err = bucket.Put(context.Background(), epochKey, []byte(epoch))
	if err != nil {
		return err
	}

	n.writeEpoch(epoch)

	return nil
}

func (n *NATS) writeEpoch(val string) {
	n.epochMu.Lock()
	defer n.epochMu.Unlock()

	n.epoch = val
	if n.local != nil {
		n.local.SetEpoch(val)
	}
}

func (n *NATS) HandleBroadcast(msg *common.StreamMessage) error {
	if msg.Meta != nil && msg.Meta.Transient {
		n.broadcaster.Broadcast(msg)
		return nil
	}

	err := n.Ready(jetstreamReadyTimeout)
	if err != nil {
		n.log.Debug("JetStream is not ready yet to publish messages, add to backlog")
		n.backlogAdd(msg)
		return nil
	}

	offset, err := n.add(msg.Stream, msg.Data)

	if err != nil {
		return errorx.Decorate(err, "failed to add message to JetStream Stream: stream=%s", msg.Stream)
	}

	msg.Epoch = n.Epoch()
	msg.Offset = offset

	n.broadcaster.Broadcast(msg)
	return nil
}

func (n *NATS) HandleCommand(msg *common.RemoteCommandMessage) error {
	n.broadcaster.BroadcastCommand(msg)
	return nil
}

func (n *NATS) Subscribe(stream string) string {
	isNew := n.tracker.Add(stream)

	if isNew {
		n.addStreamConsumer(stream)
		n.broadcaster.Subscribe(stream)
	}

	return stream
}

func (n *NATS) Unsubscribe(stream string) string {
	isLast := n.tracker.Remove(stream)

	if isLast {
		n.broadcaster.Unsubscribe(stream)
	}

	return stream
}

func (n *NATS) HistoryFrom(stream string, epoch string, offset uint64) ([]common.StreamMessage, error) {
	err := n.Ready(jetstreamReadyTimeout)
	if err != nil {
		return nil, err
	}

	n.streamSync.sync(stream)
	return n.local.HistoryFrom(stream, epoch, offset)
}

func (n *NATS) HistorySince(stream string, since int64) ([]common.StreamMessage, error) {
	err := n.Ready(jetstreamReadyTimeout)
	if err != nil {
		return nil, err
	}

	n.streamSync.sync(stream)
	return n.local.HistorySince(stream, since)
}

func (n *NATS) CommitSession(sid string, session Cacheable) error {
	err := n.Ready(jetstreamReadyTimeout)
	if err != nil {
		return err
	}

	ctx := context.Background()
	key := sessionsPrefix + sid
	data, err := session.ToCacheEntry()

	if err != nil {
		return errorx.Decorate(err, "failed to serialize session")
	}

	_, err = n.kv.Put(ctx, key, data)

	if err != nil {
		return errorx.Decorate(err, "failed to save session to NATS")
	}

	return nil
}

func (n *NATS) RestoreSession(sid string) ([]byte, error) {
	err := n.Ready(jetstreamReadyTimeout)
	if err != nil {
		return nil, err
	}

	key := sessionsPrefix + sid
	ctx := context.Background()

	entry, err := n.kv.Get(ctx, key)

	if err == jetstream.ErrKeyNotFound {
		return nil, nil
	}

	if err != nil {
		return nil, errorx.Decorate(err, "failed to restore session from NATS")
	}

	return entry.Value(), nil
}

func (n *NATS) TouchSession(sid string) error {
	err := n.Ready(jetstreamReadyTimeout)
	if err != nil {
		return err
	}

	ctx := context.Background()
	key := sessionsPrefix + sid

	entry, err := n.kv.Get(ctx, key)

	if err != nil {
		return errorx.Decorate(err, "failed to restore session from NATS")
	}

	_, err = n.kv.Put(ctx, key, entry.Value())

	if err != nil {
		return errorx.Decorate(err, "failed to touch session in NATS")
	}

	return nil
}

func (n *NATS) Reset() error {
	err := n.Ready(jetstreamReadyTimeout)
	if err != nil {
		return err
	}

	n.clientMu.Lock()
	defer n.clientMu.Unlock()

	// Delete all sessions
	if n.kv != nil {
		keys, err := n.kv.Keys(context.Background())
		if err != nil {
			if err != jetstream.ErrNoKeysFound {
				return err
			}
		}

		for _, key := range keys {
			n.kv.Delete(context.Background(), key) // nolint:errcheck
		}
	}

	lister := n.js.ListStreams(context.Background(), jetstream.WithStreamListSubject(sessionsPrefix+"*"))
	for info := range lister.Info() {
		n.js.DeleteStream(context.Background(), info.Config.Name) // nolint:errcheck
	}

	return nil
}

func (n *NATS) Peak(stream string) (*common.StreamMessage, error) {
	return nil, nil
}

func (n *NATS) PresenceAdd(stream string, sid string, pid string, info interface{}) (*common.PresenceEvent, error) {
	return nil, errors.New("presence not supported")
}

func (n *NATS) PresenceRemove(stream string, sid string) (*common.PresenceEvent, error) {
	return nil, errors.New("presence not supported")
}

func (n *NATS) PresenceInfo(stream string, opts ...PresenceInfoOption) (*common.PresenceInfo, error) {
	return nil, errors.New("presence not supported")
}

func (n *NATS) TouchPresence(sid string) error {
	return nil
}

func (n *NATS) add(stream string, data string) (uint64, error) {
	err := n.ensureStreamExists(stream)

	if err != nil {
		return 0, errorx.Decorate(err, "failed to create JetStream Stream")
	}

	ctx := context.Background()
	key := streamPrefix + stream

	// Touch on publish to make sure that the subsequent history fetch will return the latest messages
	n.streamSync.touch(stream)
	ack, err := n.js.Publish(ctx, key, []byte(data))

	if err != nil {
		return 0, errorx.Decorate(err, "failed to publish message to JetStream")
	}

	return ack.Sequence, nil
}

func (n *NATS) addStreamConsumer(stream string) {
	attempts := 5

	err := n.ensureStreamExists(stream)

	if err != nil {
		n.log.Error("failed to create JetStream stream", "stream", stream, "error", err)
		return
	}

createConsumer:
	prefixedStream := streamPrefix + stream

	_, cerr := n.jconsumers.fetch(stream, func() (jetstream.Consumer, error) { // nolint:errcheck
		cons, err := n.js.CreateConsumer(context.Background(), prefixedStream, jetstream.ConsumerConfig{
			AckPolicy: jetstream.AckNonePolicy,
		})

		if err != nil {
			n.log.Error("failed to create JetStream stream consumer", "stream", stream, "error", err)
			return nil, err
		}

		n.log.Debug("created JetStream consumer", "consumer", cons.CachedInfo().Name, "stream", stream)

		n.streamSync.touch(stream)

		batchSize := n.conf.HistoryLimit

		if batchSize == 0 {
			// TODO: what should we do if history is unlimited?
			batchSize = 100
		}

		batch, err := cons.FetchNoWait(batchSize)
		if err != nil {
			n.log.Error("failed to fetch initial messages from JetStream", "error", err)
			return nil, err
		}

		for msg := range batch.Messages() {
			n.consumeMessage(stream, msg)
		}

		_, err = cons.Consume(func(msg jetstream.Msg) {
			n.consumeMessage(stream, msg)
		})

		if err != nil {
			return nil, err
		}

		return cons, nil
	}, func(cons jetstream.Consumer) {
		name := cons.CachedInfo().Name
		n.log.Debug("deleting JetStream consumer", "consumer", name, "stream", stream)
		n.streamSync.remove(stream)
		n.js.DeleteConsumer(context.Background(), prefixedStream, name) // nolint:errcheck
	})

	if cerr != nil {
		if n.shouldRetryOnError(cerr, &attempts, 500*time.Millisecond) {
			goto createConsumer
		}
	}
}

func (n *NATS) consumeMessage(stream string, msg jetstream.Msg) {
	n.streamSync.touch(stream)

	meta, err := msg.Metadata()
	if err != nil {
		n.log.Error("failed to get JetStream message metadata", "error", err)
		return
	}

	seq := meta.Sequence.Stream
	ts := meta.Timestamp

	_, err = n.local.Store(stream, msg.Data(), seq, ts)
	if err != nil {
		n.log.Error("failed to store message in local broker", "error", err)
		return
	}
}

func (n *NATS) ensureStreamExists(stream string) error {
	prefixedStream := streamPrefix + stream
	attempts := 5

createStream:
	_, err := n.jstreams.fetch(stream, func() (string, error) {
		ctx := context.Background()

		_, err := n.js.CreateStream(ctx, jetstream.StreamConfig{
			Name:     prefixedStream,
			MaxMsgs:  int64(n.conf.HistoryLimit),
			MaxAge:   time.Duration(n.conf.HistoryTTL * int64(time.Second)),
			Replicas: 1,
		})

		if err != nil {
			// That means we updated the stream config (TTL, limit, etc.)
			if err != jetstream.ErrStreamNameAlreadyInUse {
				return "", err
			}
		}

		return stream, nil
	}, func(stream string) {})

	if err != nil {
		if n.shouldRetryOnError(err, &attempts, 500*time.Millisecond) {
			goto createStream
		}
	}

	return err
}

func (n *NATS) calculateEpoch() (string, error) {
	attempts := 5
	maybeNewEpoch, _ := nanoid.Nanoid(4)

	ttl := time.Duration(10 * int64(math.Max(float64(n.conf.HistoryTTL), float64(n.conf.SessionsTTL))*float64(time.Second)))
	// We must use a separate bucket due to a different TTL
	bucketKey := epochBucket

fetchEpoch:
	kv, err := n.fetchBucketWithTTL(bucketKey, ttl)

	if err != nil {
		return "", errorx.Decorate(err, "failed to connect to JetStream KV")
	}

	n.epochKV = kv

	_, err = kv.Create(context.Background(), epochKey, []byte(maybeNewEpoch))

	if err != nil && strings.Contains(err.Error(), "key exists") {
		entry, kerr := kv.Get(context.Background(), epochKey)

		if kerr != nil {
			return "", errorx.Decorate(kerr, "failed to retrieve key: %s", epochKey)
		}

		return string(entry.Value()), nil
	} else if err != nil {
		if n.shouldRetryOnError(err, &attempts, 1*time.Second) {
			goto fetchEpoch
		}

		return "", errorx.Decorate(err, "failed to create key: %s", epochKey)
	}

	return maybeNewEpoch, nil
}

func (n *NATS) watchEpoch(ctx context.Context) error {
	watcher, err := n.epochKV.Watch(context.Background(), epochKey, jetstream.IgnoreDeletes())

	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				watcher.Stop() // nolint:errcheck
				return
			case entry := <-watcher.Updates():
				if entry != nil {
					newEpoch := string(entry.Value())

					if n.Epoch() != newEpoch {
						n.log.Warn("epoch updated", "epoch", newEpoch)
						n.writeEpoch(newEpoch)
					}
				}
			}
		}
	}()

	return nil
}

func (n *NATS) fetchBucketWithTTL(key string, ttl time.Duration) (jetstream.KeyValue, error) {
	var bucket jetstream.KeyValue
	newBucket := true
	attempts := 5

bucketSetup:
	bucket, err := n.js.CreateKeyValue(context.Background(), jetstream.KeyValueConfig{
		Bucket:   key,
		TTL:      ttl,
		Replicas: 1,
	})

	if err != nil {
		if context.DeadlineExceeded == err {
			if attempts > 0 {
				attempts--
				n.log.Warn("failed to retrieve bucket, retrying in 500ms...", "bucket", key)
				time.Sleep(500 * time.Millisecond)
				goto bucketSetup
			}

			return nil, errorx.Decorate(err, "failed to create bucket: %s", key)
		}

		// That means that bucket has been already created
		if errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
			newBucket = false
			bucket, err = n.js.KeyValue(context.Background(), key)

			if err != nil {
				return nil, errorx.Decorate(err, "failed to retrieve bucket: %s", key)
			}
		}
	}

	if err != nil {
		return nil, errorx.Decorate(err, "failed to create bucket: %s", key)
	}

	// Invalidate TTL settings if the bucket is the new one.
	// We discard the previous bucket and create a new one with the default TTL.
	if !newBucket {
		status, serr := bucket.Status(context.Background())

		if serr != nil {
			return nil, errorx.Decorate(serr, "failed to retrieve bucket status: %s", key)
		}

		if status.TTL() != ttl {
			n.log.Warn("bucket TTL has been changed, recreating the bucket", "bucket", key, "old_ttl", status.TTL().String(), "ttl", ttl.String())
			derr := n.js.DeleteKeyValue(context.Background(), key)
			if derr != nil {
				return nil, errorx.Decorate(derr, "failed to delete bucket: %s", key)
			}

			goto bucketSetup
		}
	}

	return bucket, nil
}

type lru[T comparable] struct {
	entries map[string]lruEntry[T]
	ttl     time.Duration
	mu      sync.RWMutex
}

type lruEntry[T comparable] struct {
	value      T
	lastActive time.Time
	cleanup    func(T)
}

func newLRU[T comparable](ttl time.Duration) *lru[T] {
	return &lru[T]{entries: make(map[string]lruEntry[T]), ttl: ttl}
}

func (m *lru[T]) fetch(key string, create func() (T, error), cleanup func(T)) (T, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if val, ok := m.read(key); ok {
		return val, nil
	}

	val, err := create()

	if err != nil {
		var zero T
		return zero, err
	}

	m.write(key, val, cleanup)

	return val, nil
}

func (m *lru[T]) write(key string, value T, cleanup func(v T)) {
	m.entries[key] = lruEntry[T]{value: value, lastActive: time.Now(), cleanup: cleanup}
	// perform expiration on writes (which must happen rarely)
	m.expire()
}

func (m *lru[T]) read(key string) (res T, found bool) {
	if entry, ok := m.entries[key]; ok {
		if entry.lastActive.Add(m.ttl).Before(time.Now()) {
			return
		}

		// touch entry
		entry.lastActive = time.Now()
		res = entry.value
		found = true
	}

	return
}

func (m *lru[T]) expire() {
	for key, entry := range m.entries {
		if entry.lastActive.Add(m.ttl).Before(time.Now()) {
			delete(m.entries, key)
			entry.cleanup(entry.value)
		}
	}
}

type streamsSynchronizer struct {
	my       sync.RWMutex
	enntries map[string]*streamSync
}

func newStreamsSynchronizer() *streamsSynchronizer {
	return &streamsSynchronizer{
		enntries: make(map[string]*streamSync),
	}
}

func (s *streamsSynchronizer) sync(stream string) {
	s.my.RLock()

	syncer, ok := s.enntries[stream]

	s.my.RUnlock()

	if !ok {
		return
	}

	syncer.sync()
}

func (s *streamsSynchronizer) touch(stream string) {
	s.my.RLock()

	syncer, ok := s.enntries[stream]

	s.my.RUnlock()

	if ok {
		syncer.restart()
		return
	}

	s.my.Lock()
	defer s.my.Unlock()

	s.enntries[stream] = newStreamSync()
	s.enntries[stream].restart()
}

func (s *streamsSynchronizer) remove(stream string) {
	s.my.Lock()
	defer s.my.Unlock()

	if syncer, ok := s.enntries[stream]; ok {
		syncer.idle()
		delete(s.enntries, stream)
	}
}

type streamSync struct {
	mu          sync.Mutex
	active      bool
	activeSince time.Time

	cv    chan struct{}
	timer *time.Timer
}

const (
	streamHistorySyncTimeout = 200 * time.Millisecond
	streamHistorySyncPeriod  = 50 * time.Millisecond
)

func newStreamSync() *streamSync {
	return &streamSync{}
}

// sync waits for the gap in currently consumed messages
func (s *streamSync) sync() {
	s.mu.Lock()

	if !s.active {
		s.mu.Unlock()
		return
	}

	s.mu.Unlock()

	<-s.cv
}

// restart is called every time a message is consumed to
// keep this stream locked from reading history
func (s *streamSync) restart() {
	s.mu.Lock()

	if s.active {
		if s.activeSince.Add(streamHistorySyncTimeout).Before(time.Now()) {
			s.mu.Unlock()
			s.idle()
			return
		}
		s.timer.Reset(streamHistorySyncPeriod)
		s.mu.Unlock()
		return
	}

	defer s.mu.Unlock()

	s.active = true
	s.activeSince = time.Now()
	s.timer = time.AfterFunc(streamHistorySyncPeriod, s.idle)
	s.cv = make(chan struct{})
}

func (s *streamSync) idle() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return
	}

	s.active = false
	close(s.cv)
}

func (n *NATS) shouldRetryOnError(err error, attempts *int, cooldown time.Duration) bool {
	if context.DeadlineExceeded == err || jetstream.ErrNoStreamResponse == err {
		if *attempts > 0 {
			(*attempts)--
			n.log.Warn(fmt.Sprintf("operation failed, retrying in %s...", cooldown.String()), "error", err)
			time.Sleep(cooldown)
			return true
		}
	}

	return false
}

func (n *NATS) backlogAdd(msg *common.StreamMessage) {
	n.backlogMu.Lock()
	defer n.backlogMu.Unlock()

	n.broadcastBacklog = append(n.broadcastBacklog, msg)
}

func (n *NATS) backlogFlush() {
	n.backlogMu.Lock()
	defer n.backlogMu.Unlock()

	for _, msg := range n.broadcastBacklog {
		n.HandleBroadcast(msg) // nolint:errcheck
	}

	n.broadcastBacklog = []*common.StreamMessage{}
}
