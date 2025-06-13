package broker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"sync"

	_ "embed"

	rconfig "github.com/anycable/anycable-go/redis"
	"github.com/joomcode/errorx"

	"github.com/anycable/anycable-go/common"
	nanoid "github.com/matoous/go-nanoid"
	"github.com/redis/rueidis"
)

// RedisBroker is a broker implementation that uses Redis Streams to keep streams history
type RedisBroker struct {
	broadcaster Broadcaster
	presenter   Presenter
	conf        *Config
	tracker     *StreamsTracker
	presence    *redisPresence

	rconf       *rconfig.RedisConfig
	client      rueidis.Client
	clusterMode bool
	clientMu    sync.RWMutex
	keyMu       sync.RWMutex

	epoch     string
	addScript *rueidis.Lua

	log *slog.Logger
}

const (
	redisSessionsPrefix        = "$ac:se:"
	redisStreamPrefix          = "$ac:s:"
	redisStreamPosPrefix       = "$ac:sp:"
	redisStreamTimestampPrefix = "$ac:st:"
	redisEpochPrefix           = "$ac"
	redisEpochKey              = "e"
)

//go:embed redis/add_message.lua
var addToStreamSource string

var _ Broker = (*RedisBroker)(nil)

func NewRedisBroker(broadcaster Broadcaster, p Presenter, c *Config, rc *rconfig.RedisConfig, l *slog.Logger) *RedisBroker {
	b := RedisBroker{
		broadcaster: broadcaster,
		presenter:   p,
		conf:        c,
		rconf:       rc,
		tracker:     NewStreamsTracker(),
		log:         l.With("context", "broker").With("provider", "redis"),
	}
	b.presence = NewRedisPresence(&b)
	return &b
}

func (b *RedisBroker) Start(done chan (error)) error {
	options, err := b.rconf.ToRueidisOptions()

	if err != nil {
		return err
	}

	b.clientMu.Lock()
	defer b.clientMu.Unlock()

	c, err := rueidis.NewClient(*options)

	if err != nil {
		return err
	}

	b.client = c

	epoch, err := b.calculateEpoch()

	if err != nil {
		b.client.Close()
		return err
	}

	b.epoch = epoch

	b.log.Debug("broker epoch", "epoch", b.epoch)

	mode := b.client.Mode()

	b.log.Debug("Redis client mode", "mode", mode)

	if b.client.Mode() == rueidis.ClientModeCluster {
		b.keyMu.Lock()
		b.clusterMode = true
		b.keyMu.Unlock()
	}

	b.addScript = rueidis.NewLuaScript(addToStreamSource)
	if err := b.presence.Start(); err != nil {
		b.client.Close()
		return err
	}

	return nil
}

func (b *RedisBroker) Shutdown(ctx context.Context) error {
	b.clientMu.Lock()
	defer b.clientMu.Unlock()

	if b.client != nil {
		b.client.Close()
	}

	if err := b.presence.Stop(ctx); err != nil {
		return err
	}

	return nil
}

func (b *RedisBroker) Announce() string {
	// FIXME: This should be moved out of here
	// ensure Redis config is parsed
	b.rconf.ToRueidisOptions() // nolint:errcheck

	brokerParams := fmt.Sprintf("(history limit: %d, history ttl: %ds, sessions ttl: %ds, presence ttl: %ds)", b.conf.HistoryLimit, b.conf.HistoryTTL, b.conf.SessionsTTL, b.conf.PresenceTTL)

	if b.rconf.IsSentinel() { //nolint:gocritic
		return fmt.Sprintf("Using Redis broker at %v (sentinels) %s", b.rconf.Hostnames(), brokerParams)
	} else if b.rconf.IsCluster() {
		return fmt.Sprintf("Using Redis broker at %v (cluster) %s", b.rconf.Hostnames(), brokerParams)
	} else {
		return fmt.Sprintf("Using Redis broker at %s %s", b.rconf.Hostname(), brokerParams)
	}
}

func (b *RedisBroker) HandleBroadcast(msg *common.StreamMessage) error {
	if msg.Meta != nil && msg.Meta.Transient {
		b.broadcaster.Broadcast(msg)
		return nil
	}

	offset, err := b.add(msg.Stream, msg.Data)

	if err != nil {
		return errorx.Decorate(err, "failed to add message to Redis stream: stream=%s", msg.Stream)
	}

	msg.Epoch = b.epoch
	msg.Offset = offset

	b.broadcaster.Broadcast(msg)
	return nil
}

func (b *RedisBroker) HandleCommand(msg *common.RemoteCommandMessage) error {
	b.broadcaster.BroadcastCommand(msg)
	return nil
}

// Registring streams (for granular pub/sub)
func (b *RedisBroker) Subscribe(stream string) string {
	isNew := b.tracker.Add(stream)

	if isNew {
		b.broadcaster.Subscribe(stream)
	}

	return stream
}

func (b *RedisBroker) Unsubscribe(stream string) string {
	isLast := b.tracker.Remove(stream)

	if isLast {
		b.broadcaster.Unsubscribe(stream)
	}

	return stream
}

func (b *RedisBroker) HistoryFrom(stream string, epoch string, offset uint64) ([]common.StreamMessage, error) {
	client, err := b.Client()

	if err != nil {
		return nil, err
	}

	b.clientMu.RLock()
	if epoch != b.epoch {
		b.clientMu.RUnlock()
		return nil, errors.New("epoch mismatch")
	}
	b.clientMu.RUnlock()

	ctx := context.Background()
	key := b.toKey(redisStreamPrefix, stream)
	start := "(" + fmt.Sprintf("%d", offset)

	if !b.keyExists(ctx, key) {
		return nil, errors.New("stream does not exist")
	}

	res := client.Do(ctx, b.client.B().Xrange().Key(key).Start(start).End("+").Build())

	if res.Error() != nil {
		return nil, errorx.Decorate(res.Error(), "failed to get history from Redis")
	}

	messages, err := res.AsXRange()
	if err != nil {
		return nil, errorx.Decorate(err, "failed to parse history from Redis")
	}

	history := []common.StreamMessage{}

	for _, msg := range messages {
		data := msg.FieldValues["d"]
		parts := strings.SplitN(msg.ID, "-", 2)

		id, _ := strconv.ParseUint(parts[0], 10, 64)

		history = append(history, common.StreamMessage{
			Stream: stream,
			Data:   data,
			Offset: id,
			Epoch:  epoch,
		})
	}

	return history, nil
}

func (b *RedisBroker) HistorySince(stream string, ts int64) ([]common.StreamMessage, error) {
	client, err := b.Client()

	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	key := b.toKey(redisStreamTimestampPrefix, stream)
	// Redis uses milliseconds as message IDs
	start := fmt.Sprintf("%d", ts*1000)

	if !b.keyExists(ctx, key) {
		return nil, nil
	}

	res := client.Do(ctx, b.client.B().Xrange().Key(key).Start(start).End("+").Count(1).Build())

	if res.Error() != nil {
		return nil, errorx.Decorate(res.Error(), "failed to get history from Redis")
	}

	messages, err := res.AsXRange()
	if err != nil {
		return nil, errorx.Decorate(err, "failed to parse history from Redis")
	}

	if len(messages) == 0 {
		return []common.StreamMessage{}, nil
	}

	offset, err := strconv.ParseUint(messages[0].FieldValues["pos"], 10, 64)

	if err != nil {
		return nil, errorx.Decorate(err, "failed to parse offset from Redis response")
	}

	return b.HistoryFrom(stream, b.epoch, offset-1)
}

func (b *RedisBroker) Peak(stream string) (*common.StreamMessage, error) {
	client, err := b.Client()

	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	key := redisStreamPrefix + stream

	res := client.Do(ctx, client.B().Xrevrange().Key(key).End("+").Start("-").Count(1).Build())

	if res.Error() != nil {
		return nil, errorx.Decorate(res.Error(), "failed to get stream tail from Redis")
	}

	messages, err := res.AsXRange()
	if err != nil {
		return nil, errorx.Decorate(err, "failed to parse stream tail from Redis")
	}

	// unknown stream or empty
	if len(messages) == 0 {
		return nil, nil
	}

	parts := strings.SplitN(messages[0].ID, "-", 2)
	offset, err := strconv.ParseUint(parts[0], 10, 64)

	if err != nil {
		return nil, errorx.Decorate(err, "failed to parse offset from Redis response")
	}

	return &common.StreamMessage{Stream: stream, Offset: offset, Epoch: b.epoch}, nil
}

func (b *RedisBroker) CommitSession(sid string, session Cacheable) error {
	client, err := b.Client()

	if err != nil {
		return err
	}

	ctx := context.Background()
	key := redisSessionsPrefix + sid
	ttl := b.conf.SessionsTTL
	data, err := session.ToCacheEntry()

	if err != nil {
		return errorx.Decorate(err, "failed to serialize session")
	}

	err = client.Do(ctx, client.B().Set().Key(key).Value(string(data)).ExSeconds(ttl).Build()).Error()

	if err != nil {
		return errorx.Decorate(err, "failed to store session in Redis")
	}

	return nil
}

func (b *RedisBroker) RestoreSession(sid string) ([]byte, error) {
	client, err := b.Client()

	if err != nil {
		return nil, err
	}

	key := redisSessionsPrefix + sid
	ctx := context.Background()

	data, err := client.Do(ctx, b.client.B().Get().Key(key).Build()).ToString()

	if err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, nil
		}

		return nil, errorx.Decorate(err, "failed to restore session from Redis")
	}

	return []byte(data), nil
}

func (b *RedisBroker) TouchSession(sid string) error {
	client, err := b.Client()

	if err != nil {
		return err
	}

	ctx := context.Background()
	key := redisSessionsPrefix + sid
	ttl := b.conf.SessionsTTL

	err = client.Do(ctx, client.B().Expire().Key(key).Seconds(ttl).Build()).Error()

	if err != nil {
		return errorx.Decorate(err, "failed to delete session cache from Redis")
	}

	return nil
}

func (b *RedisBroker) Epoch() string {
	b.clientMu.RLock()
	defer b.clientMu.RUnlock()

	return b.epoch
}

func (b *RedisBroker) SetEpoch(val string) error {
	b.clientMu.RLock()
	defer b.clientMu.RUnlock()

	ctx := context.Background()
	key := b.toKey(redisEpochPrefix, redisEpochKey)

	resp := b.client.Do(ctx, b.client.B().Set().Key(key).Value(val).Build())

	if err := resp.Error(); err != nil && !rueidis.IsRedisNil(err) {
		return errorx.Decorate(err, "failed to set epoch")
	}

	b.epoch = val

	return nil
}

// Clear drops all Redis keys managed by the broker
// and recalculates the epoch
func (b *RedisBroker) Reset() error {
	b.clientMu.RLock()
	defer b.clientMu.RUnlock()

	ctx := context.Background()
	c := b.client
	msgs, err := c.Do(ctx, c.B().Keys().Pattern("$ac:*").Build()).ToArray()

	if err != nil {
		return fmt.Errorf("failed to retrieve keys: %v", err)
	}

	if b.clusterMode {
		for _, msg := range msgs {
			key, _ := msg.ToString()

			if kerr := c.Do(ctx, c.B().Del().Key(key).Build()).Error(); kerr != nil {
				return errorx.Decorate(kerr, "failed to delete key")
			}
		}
	} else {
		keys := []string{}

		for _, msg := range msgs {
			key, _ := msg.ToString()
			keys = append(keys, key)
		}

		res := c.Do(context.Background(), c.B().Del().Key(keys...).Build())
		err = res.Error()

		if err != nil {
			return errorx.Decorate(err, "failed to delete keys")
		}
	}

	epoch, err := b.calculateEpoch()

	if err != nil {
		return errorx.Decorate(err, "failed to calculate epoch")
	}

	b.epoch = epoch

	return nil
}

// Epoch is stored in Redis once and kept for a long time (10*max(history_ttl, sessions_ttl))
func (b *RedisBroker) calculateEpoch() (string, error) {
	maybeNewEpoch, _ := nanoid.Nanoid(4)

	ttl := 10 * int64(math.Max(float64(b.conf.HistoryTTL), float64(b.conf.SessionsTTL)))
	key := b.toKey(redisEpochPrefix, redisEpochKey)
	ctx := context.Background()

	for i, resp := range b.client.DoMulti(ctx,
		b.client.B().Set().Key(key).Value(maybeNewEpoch).Nx().Build(),
		b.client.B().Expire().Key(key).Seconds(ttl).Build(),
		b.client.B().Get().Key(key).Build(),
	) {
		if err := resp.Error(); err != nil && !rueidis.IsRedisNil(err) {
			return "", errorx.Decorate(err, "failed to calculate epoch")
		}

		if i == 2 {
			return resp.ToString()
		}
	}

	return "", errors.New("something went really wrong")
}

func (b *RedisBroker) add(stream string, data string) (uint64, error) {
	client, err := b.Client()

	if err != nil {
		return 0, err
	}

	ctx := context.Background()
	epochKey := b.toKey(redisEpochPrefix, redisEpochKey)
	streamKey := b.toKey(redisStreamPrefix, stream)
	posKey := b.toKey(redisStreamPosPrefix, stream)
	tsKey := b.toKey(redisStreamTimestampPrefix, stream)

	ttl := fmt.Sprintf("%d", b.conf.HistoryTTL)
	limit := fmt.Sprintf("%d", b.conf.HistoryLimit)

	res := b.addScript.Exec(ctx, client, []string{streamKey, posKey, tsKey, epochKey}, []string{data, limit, ttl})

	if res.Error() != nil {
		return 0, errorx.Decorate(res.Error(), "failed to add message to Redis stream")
	}

	offset, err := res.ToInt64()

	if err != nil {
		return 0, errorx.Decorate(err, "failed to parse offset from Redis response")
	}

	return uint64(offset), nil
}

func (b *RedisBroker) keyExists(ctx context.Context, key string) bool {
	res := b.client.Do(ctx, b.client.B().Exists().Key(key).Build())

	if res.Error() != nil {
		return false
	}

	exists, err := res.ToInt64()

	if err != nil {
		return false
	}

	return exists > 0
}

func (b *RedisBroker) Client() (rueidis.Client, error) {
	b.clientMu.RLock()
	defer b.clientMu.RUnlock()

	if b.client == nil {
		return nil, errors.New("no Redis client initialized")
	}

	return b.client, nil
}

func (b *RedisBroker) toKey(prefix string, id string, suffixes ...string) string {
	b.keyMu.RLock()
	defer b.keyMu.RUnlock()

	if b.clusterMode {
		id = fmt.Sprintf("{%s}", id)
	}

	id = fmt.Sprintf("%s:%s", prefix, id)

	for _, suffix := range suffixes {
		id = fmt.Sprintf("%s:%s", id, suffix)
	}

	return id
}

// Consider in future releases, if we need to trim everlasting streams
// func (b *RedisBroker) expire() {
// 	// Use XTRIM to remove old messages.
// 	// MINID: Redis CURRENT_TIME - HISTORY_TTL
// 	// NOTE: Redis 6.2.0+ is required for MINID
// }
