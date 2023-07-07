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

	rconfig "github.com/anycable/anycable-go/redis"
	"github.com/joomcode/errorx"

	"github.com/anycable/anycable-go/common"
	nanoid "github.com/matoous/go-nanoid"
	"github.com/redis/rueidis"
)

// RedisBroker is a broker implementation that uses Redis Streams to keep streams history
type RedisBroker struct {
	broadcaster Broadcaster
	conf        *Config
	tracker     *StreamsTracker

	rconf    *rconfig.RedisConfig
	client   rueidis.Client
	clientMu sync.RWMutex

	epoch     string
	addScript *rueidis.Lua

	log *slog.Logger
}

const (
	redisSessionsPrefix = "$ac:se:"
	redisStreamPrefix   = "$ac:s:"
	redisStreamTsPrefix = "$ac:sts:"
	redisEpochKey       = "$ac:e"
)

const (
	// This script is responsible for adding a message to the cache.
	// A separate key is used to store the current position in the stream.
	// A separate stream is used to store timestamps for positions (used by HistorySince).
	//
	// KEYS[1] - stream key
	// ARGV[1] - message payload
	// ARGV[2] - history limit
	// ARGV[3] - history ttl
	addToStreamSource = `
	local key = "$ac:s:" .. KEYS[1]
	local posKey = "$ac:spos:" .. KEYS[1]
	local tsKey = "$ac:sts:" .. KEYS[1]

	local pos = redis.call("incr", posKey)
	local posTTL = tonumber(ARGV[3]) * 10
	redis.call("expire", posKey, posTTL)

	local maxlen = ARGV[2]

	redis.call("xadd", key, "MAXLEN", maxlen, pos, "d", ARGV[1])
	redis.call("xadd", tsKey, "MAXLEN", maxlen, "*", "pos", pos)

	redis.call("expire", key, ARGV[3])
	redis.call("expire", tsKey, ARGV[3])

	return pos
	`
)

var _ Broker = (*RedisBroker)(nil)

func NewRedisBroker(broadcaster Broadcaster, c *Config, rc *rconfig.RedisConfig, l *slog.Logger) *RedisBroker {
	return &RedisBroker{
		broadcaster: broadcaster,
		conf:        c,
		rconf:       rc,
		tracker:     NewStreamsTracker(),
		log:         l.With("context", "broker").With("provider", "redis"),
	}
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

	b.addScript = rueidis.NewLuaScript(addToStreamSource)

	return nil
}

func (b *RedisBroker) Shutdown(ctx context.Context) error {
	b.clientMu.Lock()
	defer b.clientMu.Unlock()

	if b.client != nil {
		b.client.Close()
	}

	return nil
}

func (b *RedisBroker) Announce() string {
	// FIXME: This should be moved out of here
	// ensure Redis config is parsed
	b.rconf.ToRueidisOptions() // nolint:errcheck

	brokerParams := fmt.Sprintf("(history limit: %d, history ttl: %ds, sessions ttl: %ds)", b.conf.HistoryLimit, b.conf.HistoryTTL, b.conf.SessionsTTL)

	if b.rconf.IsSentinel() { //nolint:gocritic
		return fmt.Sprintf("Using Redis broker at %v (sentinels) %s", b.rconf.Hostnames(), brokerParams)
	} else if b.rconf.IsCluster() {
		return fmt.Sprintf("Using Redis broker at %v (cluster) %s", b.rconf.Hostnames(), brokerParams)
	} else {
		return fmt.Sprintf("Using Redis broker at %s %s", b.rconf.Hostname(), brokerParams)
	}
}

func (b *RedisBroker) HandleBroadcast(msg *common.StreamMessage) {
	if msg.Meta != nil && msg.Meta.Transient {
		b.broadcaster.Broadcast(msg)
		return
	}

	offset, err := b.add(msg.Stream, msg.Data)

	if err != nil {
		b.log.Error("failed to add message to Redis stream", "stream", msg.Stream, "error", err)
		return
	}

	msg.Epoch = b.epoch
	msg.Offset = offset

	b.broadcaster.Broadcast(msg)
}

func (b *RedisBroker) HandleCommand(msg *common.RemoteCommandMessage) {
	b.broadcaster.BroadcastCommand(msg)
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
	b.clientMu.RLock()

	if b.client == nil {
		b.clientMu.RUnlock()
		return nil, errors.New("no Redis client initialized")
	}

	if epoch != b.epoch {
		b.clientMu.RUnlock()
		return nil, errors.New("epoch mismatch")
	}

	b.clientMu.RUnlock()

	ctx := context.Background()
	key := redisStreamPrefix + stream
	start := "(" + fmt.Sprintf("%d", offset)

	if !b.keyExists(ctx, key) {
		return nil, errors.New("stream does not exist")
	}

	res := b.client.Do(ctx, b.client.B().Xrange().Key(key).Start(start).End("+").Build())

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
	b.clientMu.RLock()

	if b.client == nil {
		b.clientMu.RUnlock()
		return nil, errors.New("no Redis client initialized")
	}

	b.clientMu.RUnlock()

	ctx := context.Background()
	key := redisStreamTsPrefix + stream
	// Redis uses milliseconds as message IDs
	start := fmt.Sprintf("%d", ts*1000)

	if !b.keyExists(ctx, key) {
		return nil, nil
	}

	res := b.client.Do(ctx, b.client.B().Xrange().Key(key).Start(start).End("+").Count(1).Build())

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

func (b *RedisBroker) CommitSession(sid string, session Cacheable) error {
	b.clientMu.RLock()

	if b.client == nil {
		b.clientMu.RUnlock()
		return errors.New("no Redis client initialized")
	}

	b.clientMu.RUnlock()

	ctx := context.Background()
	key := redisSessionsPrefix + sid
	ttl := b.conf.SessionsTTL
	data, err := session.ToCacheEntry()

	if err != nil {
		return errorx.Decorate(err, "failed to serialize session")
	}

	err = b.client.Do(ctx, b.client.B().Set().Key(key).Value(string(data)).ExSeconds(ttl).Build()).Error()

	if err != nil {
		return errorx.Decorate(err, "failed to store session in Redis")
	}

	return nil
}

func (b *RedisBroker) RestoreSession(sid string) ([]byte, error) {
	b.clientMu.RLock()

	if b.client == nil {
		b.clientMu.RUnlock()
		return nil, errors.New("no Redis client initialized")
	}

	b.clientMu.RUnlock()

	key := redisSessionsPrefix + sid
	ctx := context.Background()

	data, err := b.client.Do(ctx, b.client.B().Get().Key(key).Build()).ToString()

	if err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, nil
		}

		return nil, errorx.Decorate(err, "failed to restore session from Redis")
	}

	return []byte(data), nil
}

func (b *RedisBroker) FinishSession(sid string) error {
	b.clientMu.RLock()

	if b.client == nil {
		b.clientMu.RUnlock()
		return errors.New("no Redis client initialized")
	}

	b.clientMu.RUnlock()

	ctx := context.Background()
	key := redisSessionsPrefix + sid
	ttl := b.conf.SessionsTTL

	err := b.client.Do(ctx, b.client.B().Expire().Key(key).Seconds(ttl).Build()).Error()

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
	key := redisEpochKey

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
	key := redisEpochKey
	ctx := context.Background()

	for i, resp := range b.client.DoMulti(ctx,
		b.client.B().Set().Key(key).Value(maybeNewEpoch).Nx().Build(),
		b.client.B().Expire().Key(key).Seconds(ttl).Build(),
		b.client.B().Get().Key(redisEpochKey).Build(),
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
	b.clientMu.RLock()

	if b.client == nil {
		b.clientMu.RUnlock()
		return 0, errors.New("no Redis client initialized")
	}

	b.clientMu.RUnlock()

	ctx := context.Background()
	key := stream
	ttl := fmt.Sprintf("%d", b.conf.HistoryTTL)
	limit := fmt.Sprintf("%d", b.conf.HistoryLimit)

	res := b.addScript.Exec(ctx, b.client, []string{key}, []string{data, limit, ttl})

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

// Consider in future releases, if we need to trim everlasting streams
// func (b *RedisBroker) expire() {
// 	// Use XTRIM to remove old messages.
// 	// MINID: Redis CURRENT_TIME - HISTORY_TTL
// 	// NOTE: Redis 6.2.0+ is required for MINID
// }
