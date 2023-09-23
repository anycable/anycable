package broker

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/anycable/anycable-go/common"
	natsconfig "github.com/anycable/anycable-go/nats"
	"github.com/apex/log"
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

	js jetstream.JetStream
	kv jetstream.KeyValue

	clientMu sync.RWMutex

	epoch string

	log *log.Entry
}

const (
	kvBucket        = "_anycable_"
	epochBucket     = "_anycable_epoch_"
	epochKey        = "_epoch_"
	sessionsPrefix  = "ac/se/"
	streamPrefix    = "ac/s/"
	streamPosPrefix = "ac/spos/"
	streamTsPrefix  = "ac/sts/"
)

var _ Broker = (*NATS)(nil)

func NewNATSBroker(broadcaster Broadcaster, c *Config, nc *natsconfig.NATSConfig) *NATS {
	return &NATS{
		broadcaster: broadcaster,
		conf:        c,
		nconf:       nc,
		tracker:     NewStreamsTracker(),
		log:         log.WithField("context", "broker").WithField("provider", "nats"),
	}
}

// Write Broker implementtaion here
func (n *NATS) Start() error {
	n.clientMu.Lock()
	defer n.clientMu.Unlock()

	connectOptions := []nats.Option{
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(n.nconf.MaxReconnectAttempts),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				n.log.Warnf("Connection failed: %v", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			n.log.Infof("Connection restored: %s", nc.ConnectedUrl())
		}),
	}

	if n.nconf.DontRandomizeServers {
		connectOptions = append(connectOptions, nats.DontRandomize())
	}

	nc, err := nats.Connect(n.nconf.Servers, connectOptions...)

	if err != nil {
		return err
	}

	js, err := jetstream.New(nc)

	if err != nil {
		return errorx.Decorate(err, "Failed to connect to JetStream")
	}

	n.conn = nc
	n.js = js

	kv, err := n.fetchBucketWithTTL(kvBucket, time.Duration(n.conf.SessionsTTL*int64(time.Second)))

	if err != nil {
		return errorx.Decorate(err, "Failed to connect to JetStream KV")
	}

	n.kv = kv

	epoch, err := n.calculateEpoch()

	if err != nil {
		return errorx.Decorate(err, "Failed to calculate epoch")
	}

	n.epoch = epoch

	n.log.Debugf("Current epoch: %s", n.epoch)

	return nil
}

func (n *NATS) Shutdown(ctx context.Context) error {
	n.clientMu.Lock()
	defer n.clientMu.Unlock()

	if n.conn != nil {
		n.conn.Close()
		n.conn = nil
	}

	return nil
}

func (n *NATS) Announce() string {
	brokerParams := fmt.Sprintf("(history limit: %d, history ttl: %ds, sessions ttl: %ds)", n.conf.HistoryLimit, n.conf.HistoryTTL, n.conf.SessionsTTL)

	return fmt.Sprintf("Starting NATS broker: %s %s", n.nconf.Servers, brokerParams)
}

func (n *NATS) Epoch() string {
	n.clientMu.RLock()
	defer n.clientMu.RUnlock()

	return n.epoch
}

func (n *NATS) HandleBroadcast(msg *common.StreamMessage) {
	offset, err := n.add(msg.Stream, msg.Data)

	if err != nil {
		n.log.WithError(err).Errorf("failed to add message to JetStream Stream %s", msg.Stream)
		return
	}

	msg.Epoch = n.epoch
	msg.Offset = offset

	if n.tracker.Has(msg.Stream) {
		n.broadcaster.Broadcast(msg)
	}
}

func (n *NATS) HandleCommand(msg *common.RemoteCommandMessage) {
	n.broadcaster.BroadcastCommand(msg)
}

func (n *NATS) Subscribe(stream string) string {
	isNew := n.tracker.Add(stream)

	if isNew {
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
	return nil, nil
}

func (n *NATS) HistorySince(stream string, since int64) ([]common.StreamMessage, error) {
	return nil, nil
}

func (n *NATS) CommitSession(sid string, session Cacheable) error {
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

func (n *NATS) FinishSession(sid string) error {
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

func (n *NATS) add(stream string, data string) (uint64, error) {
	return 0, nil
}

func (n *NATS) calculateEpoch() (string, error) {
	maybeNewEpoch, _ := nanoid.Nanoid(4)

	ttl := time.Duration(10 * int64(math.Max(float64(n.conf.HistoryTTL), float64(n.conf.SessionsTTL))*float64(time.Second)))
	// We must use a separate bucket due to a different TTL
	bucketKey := epochBucket

	kv, err := n.fetchBucketWithTTL(bucketKey, ttl)

	if err != nil {
		return "", errorx.Decorate(err, "Failed to connect to JetStream KV")
	}

	entry, err := kv.Get(context.Background(), epochKey)

	if err == jetstream.ErrKeyNotFound {
		_, perr := kv.Put(context.Background(), epochKey, []byte(maybeNewEpoch))

		if perr != nil {
			return "", errorx.Decorate(perr, "Failed to save JetStream KV epoch")
		}

		return maybeNewEpoch, nil
	} else if err != nil {
		return "", errorx.Decorate(err, "Failed to retrieve JetStream KV epoch")
	}

	return string(entry.Value()), nil
}

func (n *NATS) fetchBucketWithTTL(key string, ttl time.Duration) (jetstream.KeyValue, error) {
	var bucket jetstream.KeyValue
	newBucket := false

bucketSetup:
	bucket, err := n.js.KeyValue(context.Background(), key)

	if err == jetstream.ErrBucketNotFound {
		var berr error
		bucket, berr = n.js.CreateKeyValue(context.Background(), jetstream.KeyValueConfig{
			Bucket: kvBucket,
			TTL:    time.Duration(n.conf.SessionsTTL * int64(time.Second)),
		})

		if berr != nil {
			return nil, errorx.Decorate(berr, "Failed to create JetStream KV bucket: %s", key)
		}

		newBucket = true
	} else if err != nil {
		return nil, errorx.Decorate(err, "Failed to retrieve JetStream KV bucket: %s", key)
	}

	// Invalidate TTL settings if the bucket is the new one.
	// We discard the previous bucket and create a new one with the default TTL.
	if !newBucket {
		status, serr := bucket.Status(context.Background())

		if serr != nil {
			return nil, errorx.Decorate(serr, "Failed to retrieve JetStream KV bucket status: %s", key)
		}

		if status.TTL() != ttl {
			n.log.Warnf("JetStream KV bucket TTL has been changed, recreating the bucket: key=%s, old=%s, new=%s", key, status.TTL().String(), ttl.String())
			derr := n.js.DeleteKeyValue(context.Background(), kvBucket)
			if derr != nil {
				return nil, errorx.Decorate(derr, "Failed to delete JetStream KV bucket: %s", key)
			}

			goto bucketSetup
		}
	}

	return bucket, nil
}
