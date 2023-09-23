package broker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/anycable/anycable-go/common"
	natsconfig "github.com/anycable/anycable-go/nats"
	"github.com/apex/log"
	"github.com/joomcode/errorx"
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
	sessionsPrefix  = "ac/se/"
	streamPrefix    = "ac/s/"
	streamPosPrefix = "ac/spos/"
	streamTsPrefix  = "ac/sts/"
	epochKey        = "ac/e"
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

	// Setup KV bucket
	var bucket jetstream.KeyValue
	newBucket := false

bucketSetup:
	bucket, err = n.js.KeyValue(context.Background(), kvBucket)

	if err == jetstream.ErrBucketNotFound {
		var berr error
		bucket, berr = n.js.CreateKeyValue(context.Background(), jetstream.KeyValueConfig{
			Bucket: kvBucket,
			TTL:    time.Duration(n.conf.SessionsTTL * int64(time.Second)),
		})

		if berr != nil {
			return errorx.Decorate(berr, "Failed to create JetStream KV bucket")
		}

		newBucket = true
	} else if err != nil {
		return errorx.Decorate(err, "Failed to retrieve JetStream KV bucket")
	}

	// Invalidate TTL settings if the bucket is the new one.
	// We discard the previous bucket and create a new one with the default TTL.
	if !newBucket {
		status, serr := bucket.Status(context.Background())

		if serr != nil {
			return errorx.Decorate(serr, "Failed to retrieve JetStream KV bucket status")
		}

		ttl := time.Duration(n.conf.SessionsTTL * int64(time.Second))

		if status.TTL() != ttl {
			n.log.Warnf("JetStream KV bucket TTL has been changed, recreating the bucket: old=%s, new=%s", status.TTL().String(), ttl.String())
			derr := n.js.DeleteKeyValue(context.Background(), kvBucket)
			if derr != nil {
				return errorx.Decorate(derr, "Failed to delete JetStream KV bucket")
			}

			goto bucketSetup
		}
	}

	n.kv = bucket

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

	return []byte(entry.Value()), nil
}

func (n *NATS) FinishSession(sid string) error {
	ctx := context.Background()
	key := sessionsPrefix + sid

	entry, err := n.kv.Get(ctx, key)

	if err != nil {
		return errorx.Decorate(err, "failed to restore session from NATS")
	}

	_, err = n.kv.Put(ctx, key, []byte(entry.Value()))

	if err != nil {
		return errorx.Decorate(err, "failed to touch session in NATS")
	}

	return nil
}

func (n *NATS) add(stream string, data string) (uint64, error) {
	return 0, nil
}
