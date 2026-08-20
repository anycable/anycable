package broker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anycable/anycable-go/common"
	redisconfig "github.com/anycable/anycable-go/redis"
	"github.com/joomcode/errorx"
	nanoid "github.com/matoous/go-nanoid"
	goredis "github.com/redis/go-redis/v9"
)

const (
	redisBrokerOperationTimeout = 5 * time.Second
	redisBrokerWatchRetries     = 32
	redisStreamSequenceBits     = 20
	redisStreamSequenceMask     = uint64(1<<redisStreamSequenceBits) - 1
)

type RedisBrokerOption func(*RedisBroker)

func WithRedisBrokerPrefix(prefix string) RedisBrokerOption {
	return func(b *RedisBroker) {
		b.prefix = prefix
	}
}

// RedisBroker stores stream history, resumable sessions, and presence in
// Redis so those features remain consistent across AnyCable nodes.
//
// The implementation intentionally relies on Redis primitives and optimistic
// WATCH/MULTI/EXEC transactions. The command set is supported by both Redis
// and Dragonfly.
type RedisBroker struct {
	broadcaster Broadcaster
	presenter   Presenter
	config      *Config
	client      goredis.UniversalClient
	tracker     *StreamsTracker
	prefix      string
	baseKey     string

	epochMu sync.RWMutex
	epoch   string

	shutdownCtx context.Context
	shutdown    context.CancelFunc
	log         *slog.Logger
}

type redisHistoryEntry struct {
	message   common.StreamMessage
	timestamp int64
}

var _ Broker = (*RedisBroker)(nil)

func NewRedisBroker(
	broadcaster Broadcaster,
	presenter Presenter,
	config *Config,
	redisConfig *redisconfig.RedisConfig,
	logger *slog.Logger,
	options ...RedisBrokerOption,
) (*RedisBroker, error) {
	redisOptions, err := redisConfig.ToGoRedisUniversalOptions()
	if err != nil {
		return nil, errorx.Decorate(err, "failed to configure Redis broker")
	}

	shutdownCtx, shutdown := context.WithCancel(context.Background())
	instance := &RedisBroker{
		broadcaster: broadcaster,
		presenter:   presenter,
		config:      config,
		client:      goredis.NewUniversalClient(redisOptions),
		tracker:     NewStreamsTracker(),
		prefix:      config.RedisPrefix,
		shutdownCtx: shutdownCtx,
		shutdown:    shutdown,
		log:         logger.With("context", "broker", "provider", "redis"),
	}

	for _, option := range options {
		option(instance)
	}

	if instance.prefix == "" {
		instance.prefix = "__anycable_broker__"
	}

	// Keep all broker keys in the same Redis Cluster/Dragonfly Cluster slot so
	// multi-key transactions remain valid.
	digest := sha256.Sum256([]byte(instance.prefix))
	instance.baseKey = fmt.Sprintf("%s:{%s}", instance.prefix, hex.EncodeToString(digest[:8]))

	return instance, nil
}

func (b *RedisBroker) Start(_ chan error) error {
	ctx, cancel := context.WithTimeout(context.Background(), redisBrokerOperationTimeout)
	defer cancel()

	if err := b.client.Ping(ctx).Err(); err != nil {
		return errorx.Decorate(err, "failed to connect Redis broker")
	}
	if err := b.initializeEpoch(ctx); err != nil {
		return err
	}
	if err := b.initializeSessionConfig(ctx); err != nil {
		return err
	}

	go b.expirePresenceLoop()
	return nil
}

func (b *RedisBroker) Shutdown(_ context.Context) error {
	b.shutdown()
	return b.client.Close()
}

func (b *RedisBroker) Announce() string {
	return fmt.Sprintf(
		"Using Redis broker (history limit: %d, history ttl: %ds, sessions ttl: %ds, presence ttl: %ds, prefix: %s)",
		b.config.HistoryLimit,
		b.config.HistoryTTL,
		b.config.SessionsTTL,
		b.config.PresenceTTL,
		b.prefix,
	)
}

func (b *RedisBroker) HandleBroadcast(msg *common.StreamMessage) error {
	if msg.Meta != nil && msg.Meta.Transient {
		b.broadcaster.Broadcast(msg)
		return nil
	}

	ctx := context.Background()
	key := b.historyKey(msg.Stream)
	var add *goredis.StringCmd
	_, err := b.client.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
		add = pipe.XAdd(ctx, &goredis.XAddArgs{
			Stream: key,
			MaxLen: int64(b.config.HistoryLimit),
			Values: map[string]interface{}{"data": msg.Data},
		})
		pipe.Expire(ctx, key, time.Duration(b.config.HistoryTTL)*time.Second)
		return nil
	})
	if err != nil {
		return errorx.Decorate(err, "failed to store Redis broker history: stream=%s", msg.Stream)
	}

	offset, _, err := redisStreamIDToOffset(add.Val())
	if err != nil {
		return errorx.Decorate(err, "failed to parse Redis broker history offset")
	}

	msg.Offset = offset
	msg.Epoch = b.Epoch()
	b.broadcaster.Broadcast(msg)
	return nil
}

func (b *RedisBroker) HandleCommand(msg *common.RemoteCommandMessage) error {
	b.broadcaster.BroadcastCommand(msg)
	return nil
}

func (b *RedisBroker) Subscribe(stream string) string {
	if b.tracker.Add(stream) {
		b.broadcaster.Subscribe(stream)
	}
	return stream
}

func (b *RedisBroker) Unsubscribe(stream string) string {
	if b.tracker.Remove(stream) {
		b.broadcaster.Unsubscribe(stream)
	}
	return stream
}

func (b *RedisBroker) HistoryFrom(stream string, epoch string, offset uint64) ([]common.StreamMessage, error) {
	currentEpoch := b.Epoch()
	if currentEpoch != epoch {
		return nil, fmt.Errorf("unknown epoch: %s, current: %s", epoch, currentEpoch)
	}

	key := b.historyKey(stream)
	found, err := b.client.Exists(context.Background(), key).Result()
	if err != nil {
		return nil, err
	}
	if found == 0 {
		return nil, errors.New("stream not found")
	}
	if offset != 0 {
		id := redisOffsetToStreamID(offset)
		position, positionErr := b.client.XRangeN(context.Background(), key, id, id, 1).Result()
		if positionErr != nil {
			return nil, positionErr
		}
		if len(position) == 0 {
			return nil, fmt.Errorf("requested offset couldn't be found: %d", offset)
		}
	}

	entries, err := b.readHistory(stream, "("+redisOffsetToStreamID(offset), "+")
	if err != nil {
		return nil, err
	}

	history := make([]common.StreamMessage, 0, len(entries))
	for _, entry := range entries {
		history = append(history, entry.message)
	}
	return history, nil
}

func (b *RedisBroker) HistorySince(stream string, since int64) ([]common.StreamMessage, error) {
	cutoff := since
	ttlCutoff := time.Now().Unix() - b.config.HistoryTTL
	if ttlCutoff > cutoff {
		cutoff = ttlCutoff
	}

	entries, err := b.readHistory(stream, fmt.Sprintf("%d-0", cutoff*1000), "+")
	if err != nil {
		return nil, err
	}

	history := make([]common.StreamMessage, 0, len(entries))
	for _, entry := range entries {
		history = append(history, entry.message)
	}
	return history, nil
}

func (b *RedisBroker) Peak(stream string) (*common.StreamMessage, error) {
	messages, err := b.client.XRevRangeN(context.Background(), b.historyKey(stream), "+", "-", 1).Result()
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, nil
	}

	offset, timestamp, err := redisStreamIDToOffset(messages[0].ID)
	if err != nil {
		return nil, err
	}
	if timestamp < time.Now().Unix()-b.config.HistoryTTL {
		return nil, nil
	}
	return &common.StreamMessage{Offset: offset, Epoch: b.Epoch()}, nil
}

func (b *RedisBroker) CommitSession(sid string, session Cacheable) error {
	generation, ttl, err := b.sessionConfig(context.Background())
	if err != nil {
		return err
	}
	if ttl <= 0 {
		return nil
	}

	data, err := session.ToCacheEntry()
	if err != nil {
		return errorx.Decorate(err, "failed to serialize session")
	}
	return b.client.Set(context.Background(), b.sessionKey(sid, generation), data, time.Duration(ttl)*time.Second).Err()
}

func (b *RedisBroker) RestoreSession(sid string) ([]byte, error) {
	generation, ttl, err := b.sessionConfig(context.Background())
	if err != nil {
		return nil, err
	}
	if ttl <= 0 {
		return nil, nil
	}

	data, err := b.client.Get(context.Background(), b.sessionKey(sid, generation)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	return data, err
}

func (b *RedisBroker) TouchSession(sid string) error {
	generation, ttl, err := b.sessionConfig(context.Background())
	if err != nil {
		return err
	}
	if ttl <= 0 {
		return nil
	}
	return b.client.Expire(context.Background(), b.sessionKey(sid, generation), time.Duration(ttl)*time.Second).Err()
}

func (b *RedisBroker) SupportsPresence() bool {
	return true
}

func (b *RedisBroker) PresenceAdd(stream string, sid string, pid string, info interface{}) (*common.PresenceEvent, error) {
	token := redisKeyDigest(stream)
	b.cleanupPresenceStream(context.Background(), stream, token)

	encodedInfo, err := json.Marshal(info)
	if err != nil {
		return nil, errorx.Decorate(err, "failed to serialize presence info")
	}

	keys := b.presenceStreamKeys(token)
	sessionKey := b.presenceSessionKey(sid)
	watchKeys := append(append([]string{}, keys...), sessionKey)
	joined := false
	err = b.watch(context.Background(), watchKeys, func(tx *goredis.Tx) error {
		joined = false
		existingPID, getErr := tx.HGet(context.Background(), keys[0], sid).Result()
		if getErr != nil && !errors.Is(getErr, goredis.Nil) {
			return getErr
		}
		if getErr == nil && existingPID != pid {
			return errors.New("presence ID mismatch")
		}

		count := int64(0)
		newMember := errors.Is(getErr, goredis.Nil)
		if newMember {
			count, getErr = tx.HGet(context.Background(), keys[1], pid).Int64()
			if getErr != nil && !errors.Is(getErr, goredis.Nil) {
				return getErr
			}
			joined = count == 0
		}

		deadline := float64(time.Now().Add(time.Duration(b.config.PresenceTTL) * time.Second).UnixMilli())
		_, txErr := tx.TxPipelined(context.Background(), func(pipe goredis.Pipeliner) error {
			if newMember {
				pipe.HSet(context.Background(), keys[0], sid, pid)
				pipe.HIncrBy(context.Background(), keys[1], pid, 1)
			}
			pipe.HSet(context.Background(), keys[2], pid, encodedInfo)
			pipe.ZAdd(context.Background(), keys[3], goredis.Z{Score: deadline, Member: sid})
			pipe.SAdd(context.Background(), sessionKey, token)
			pipe.Expire(context.Background(), sessionKey, time.Duration(b.config.PresenceTTL*10)*time.Second)
			pipe.SAdd(context.Background(), b.presenceIndexKey(), token)
			pipe.HSet(context.Background(), b.presenceStreamNamesKey(), token, stream)
			return nil
		})
		return txErr
	})
	if err != nil {
		return nil, err
	}

	if !joined {
		return nil, nil
	}

	event := &common.PresenceEvent{Type: common.PresenceJoinType, ID: pid, Info: info}
	if b.presenter != nil {
		b.presenter.HandleJoin(stream, event)
	}
	return event, nil
}

func (b *RedisBroker) PresenceRemove(stream string, sid string) (*common.PresenceEvent, error) {
	token := redisKeyDigest(stream)
	b.cleanupPresenceStream(context.Background(), stream, token)

	keys := b.presenceStreamKeys(token)
	sessionKey := b.presenceSessionKey(sid)
	watchKeys := append(append([]string{}, keys...), sessionKey)
	leftPID := ""
	err := b.watch(context.Background(), watchKeys, func(tx *goredis.Tx) error {
		leftPID = ""
		pid, getErr := tx.HGet(context.Background(), keys[0], sid).Result()
		if errors.Is(getErr, goredis.Nil) {
			return errors.New("presence info not found")
		}
		if getErr != nil {
			return getErr
		}

		count, getErr := tx.HGet(context.Background(), keys[1], pid).Int64()
		if getErr != nil {
			return getErr
		}
		if count <= 1 {
			leftPID = pid
		}
		memberTotal, getErr := tx.HLen(context.Background(), keys[0]).Result()
		if getErr != nil {
			return getErr
		}

		_, txErr := tx.TxPipelined(context.Background(), func(pipe goredis.Pipeliner) error {
			pipe.HDel(context.Background(), keys[0], sid)
			pipe.ZRem(context.Background(), keys[3], sid)
			pipe.SRem(context.Background(), sessionKey, token)
			if count <= 1 {
				pipe.HDel(context.Background(), keys[1], pid)
				pipe.HDel(context.Background(), keys[2], pid)
			} else {
				pipe.HIncrBy(context.Background(), keys[1], pid, -1)
			}
			if memberTotal <= 1 {
				pipe.Del(context.Background(), keys...)
				pipe.SRem(context.Background(), b.presenceIndexKey(), token)
				pipe.HDel(context.Background(), b.presenceStreamNamesKey(), token)
			}
			return nil
		})
		return txErr
	})
	if err != nil {
		return nil, err
	}

	if leftPID == "" {
		return nil, nil
	}

	event := &common.PresenceEvent{Type: common.PresenceLeaveType, ID: leftPID}
	if b.presenter != nil {
		b.presenter.HandleLeave(stream, event)
	}
	return event, nil
}

func (b *RedisBroker) PresenceInfo(stream string, options ...PresenceInfoOption) (*common.PresenceInfo, error) {
	token := redisKeyDigest(stream)
	b.cleanupPresenceStream(context.Background(), stream, token)
	keys := b.presenceStreamKeys(token)

	pipe := b.client.Pipeline()
	countsCmd := pipe.HGetAll(context.Background(), keys[1])
	infosCmd := pipe.HGetAll(context.Background(), keys[2])
	if _, err := pipe.Exec(context.Background()); err != nil {
		return nil, err
	}
	counts := countsCmd.Val()
	infos := infosCmd.Val()

	opts := NewPresenceInfoOptions()
	for _, option := range options {
		option(opts)
	}

	result := common.NewPresenceInfo()
	result.Total = len(counts)
	if !opts.ReturnRecords {
		return result, nil
	}

	result.Records = make([]*common.PresenceEvent, 0, len(counts))
	for pid := range counts {
		var info interface{}
		if encoded, ok := infos[pid]; ok {
			if err := json.Unmarshal([]byte(encoded), &info); err != nil {
				return nil, errorx.Decorate(err, "failed to deserialize presence info")
			}
		}
		result.Records = append(result.Records, &common.PresenceEvent{ID: pid, Info: info})
	}
	return result, nil
}

func (b *RedisBroker) TouchPresence(sid string) error {
	sessionKey := b.presenceSessionKey(sid)
	tokens, err := b.client.SMembers(context.Background(), sessionKey).Result()
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	deadline := float64(time.Now().Add(time.Duration(b.config.PresenceTTL) * time.Second).UnixMilli())
	_, err = b.client.TxPipelined(context.Background(), func(pipe goredis.Pipeliner) error {
		for _, token := range tokens {
			pipe.ZAddArgs(context.Background(), b.presenceStreamKeys(token)[3], goredis.ZAddArgs{
				XX:      true,
				Members: []goredis.Z{{Score: deadline, Member: sid}},
			})
		}
		pipe.Expire(context.Background(), sessionKey, time.Duration(b.config.PresenceTTL*10)*time.Second)
		return nil
	})
	return err
}

func (b *RedisBroker) Epoch() string {
	value, err := b.client.Get(context.Background(), b.epochKey()).Result()
	if err == nil {
		b.epochMu.Lock()
		b.epoch = value
		b.epochMu.Unlock()
	}

	b.epochMu.RLock()
	defer b.epochMu.RUnlock()
	return b.epoch
}

func (b *RedisBroker) SetEpoch(epoch string) error {
	if err := b.client.Set(context.Background(), b.epochKey(), epoch, 0).Err(); err != nil {
		return err
	}
	b.epochMu.Lock()
	b.epoch = epoch
	b.epochMu.Unlock()
	return nil
}

func (b *RedisBroker) Reset() error {
	if err := b.DeleteNamespace(context.Background()); err != nil {
		return err
	}
	if err := b.initializeEpoch(context.Background()); err != nil {
		return err
	}
	return b.initializeSessionConfig(context.Background())
}

func (b *RedisBroker) DeleteNamespace(ctx context.Context) error {
	var cursor uint64
	for {
		keys, next, err := b.client.Scan(ctx, cursor, b.baseKey+"*", 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := b.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

func (b *RedisBroker) initializeEpoch(ctx context.Context) error {
	epoch, err := nanoid.Nanoid(4)
	if err != nil {
		return err
	}
	if err := b.client.SetNX(ctx, b.epochKey(), epoch, 0).Err(); err != nil {
		return err
	}
	value, err := b.client.Get(ctx, b.epochKey()).Result()
	if err != nil {
		return err
	}
	b.epochMu.Lock()
	b.epoch = value
	b.epochMu.Unlock()
	return nil
}

func (b *RedisBroker) initializeSessionConfig(ctx context.Context) error {
	key := b.sessionsMetaKey()
	return b.watch(ctx, []string{key}, func(tx *goredis.Tx) error {
		values, err := tx.HGetAll(ctx, key).Result()
		if err != nil {
			return err
		}

		generation := int64(1)
		if raw := values["generation"]; raw != "" {
			generation, err = strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return err
			}
		}
		if raw := values["ttl"]; raw != "" {
			currentTTL, parseErr := strconv.ParseInt(raw, 10, 64)
			if parseErr != nil {
				return parseErr
			}
			if currentTTL == b.config.SessionsTTL {
				return nil
			}
			generation++
		}

		_, err = tx.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
			pipe.HSet(ctx, key, "generation", generation, "ttl", b.config.SessionsTTL)
			return nil
		})
		return err
	})
}

func (b *RedisBroker) sessionConfig(ctx context.Context) (int64, int64, error) {
	values, err := b.client.HMGet(ctx, b.sessionsMetaKey(), "generation", "ttl").Result()
	if err != nil {
		return 0, 0, err
	}
	if len(values) != 2 || values[0] == nil || values[1] == nil {
		if err := b.initializeSessionConfig(ctx); err != nil {
			return 0, 0, err
		}
		values, err = b.client.HMGet(ctx, b.sessionsMetaKey(), "generation", "ttl").Result()
		if err != nil {
			return 0, 0, err
		}
	}

	generation, err := strconv.ParseInt(fmt.Sprint(values[0]), 10, 64)
	if err != nil {
		return 0, 0, err
	}
	ttl, err := strconv.ParseInt(fmt.Sprint(values[1]), 10, 64)
	return generation, ttl, err
}

func (b *RedisBroker) readHistory(stream string, start string, end string) ([]redisHistoryEntry, error) {
	messages, err := b.client.XRange(context.Background(), b.historyKey(stream), start, end).Result()
	if err != nil && !errors.Is(err, goredis.Nil) {
		return nil, err
	}

	cutoff := time.Now().Unix() - b.config.HistoryTTL
	epoch := b.Epoch()
	entries := make([]redisHistoryEntry, 0, len(messages))
	for _, item := range messages {
		offset, timestamp, parseErr := redisStreamIDToOffset(item.ID)
		if parseErr != nil {
			return nil, parseErr
		}
		if timestamp < cutoff {
			continue
		}
		entries = append(entries, redisHistoryEntry{
			message: common.StreamMessage{
				Stream: stream,
				Data:   fmt.Sprint(item.Values["data"]),
				Offset: offset,
				Epoch:  epoch,
			},
			timestamp: timestamp,
		})
	}
	return entries, nil
}

func (b *RedisBroker) watch(ctx context.Context, keys []string, fn func(*goredis.Tx) error) error {
	for range redisBrokerWatchRetries {
		err := b.client.Watch(ctx, fn, keys...)
		if errors.Is(err, goredis.TxFailedErr) {
			continue
		}
		return err
	}
	return errors.New("Redis broker transaction retries exhausted")
}

func (b *RedisBroker) expirePresenceLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-b.shutdownCtx.Done():
			return
		case <-ticker.C:
			b.expirePresence(context.Background())
		}
	}
}

func (b *RedisBroker) expirePresence(ctx context.Context) {
	tokens, err := b.client.SMembers(ctx, b.presenceIndexKey()).Result()
	if err != nil {
		b.log.Warn("failed to list Redis presence streams", "error", err)
		return
	}
	for _, token := range tokens {
		stream, streamErr := b.client.HGet(ctx, b.presenceStreamNamesKey(), token).Result()
		if streamErr != nil {
			continue
		}
		b.cleanupPresenceStream(ctx, stream, token)
	}
}

func (b *RedisBroker) cleanupPresenceStream(ctx context.Context, stream string, token string) {
	keys := b.presenceStreamKeys(token)
	expired, err := b.client.ZRangeByScore(ctx, keys[3], &goredis.ZRangeBy{
		Min: "-inf",
		Max: strconv.FormatInt(time.Now().UnixMilli(), 10),
	}).Result()
	if err != nil || len(expired) == 0 {
		return
	}

	watchKeys := append([]string{}, keys...)
	for _, sid := range expired {
		watchKeys = append(watchKeys, b.presenceSessionKey(sid))
	}

	var leaves []string
	err = b.watch(ctx, watchKeys, func(tx *goredis.Tx) error {
		leaves = leaves[:0]
		stillExpired, txErr := tx.ZRangeByScore(ctx, keys[3], &goredis.ZRangeBy{
			Min: "-inf",
			Max: strconv.FormatInt(time.Now().UnixMilli(), 10),
		}).Result()
		if txErr != nil {
			return txErr
		}
		if len(stillExpired) == 0 {
			return nil
		}

		members, txErr := tx.HMGet(ctx, keys[0], stillExpired...).Result()
		if txErr != nil {
			return txErr
		}
		decrements := make(map[string]int64)
		removedCount := int64(0)
		for _, member := range members {
			if member != nil {
				decrements[fmt.Sprint(member)]++
				removedCount++
			}
		}
		memberTotal, txErr := tx.HLen(ctx, keys[0]).Result()
		if txErr != nil {
			return txErr
		}

		newCounts := make(map[string]int64, len(decrements))
		for pid, decrement := range decrements {
			count, countErr := tx.HGet(ctx, keys[1], pid).Int64()
			if countErr != nil && !errors.Is(countErr, goredis.Nil) {
				return countErr
			}
			newCounts[pid] = count - decrement
		}

		_, txErr = tx.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
			pipe.HDel(ctx, keys[0], stillExpired...)
			membersToRemove := make([]interface{}, 0, len(stillExpired))
			for _, sid := range stillExpired {
				membersToRemove = append(membersToRemove, sid)
				pipe.SRem(ctx, b.presenceSessionKey(sid), token)
			}
			pipe.ZRem(ctx, keys[3], membersToRemove...)
			for pid, count := range newCounts {
				if count <= 0 {
					pipe.HDel(ctx, keys[1], pid)
					pipe.HDel(ctx, keys[2], pid)
					leaves = append(leaves, pid)
				} else {
					pipe.HSet(ctx, keys[1], pid, count)
				}
			}
			if memberTotal <= removedCount {
				pipe.Del(ctx, keys...)
				pipe.SRem(ctx, b.presenceIndexKey(), token)
				pipe.HDel(ctx, b.presenceStreamNamesKey(), token)
			}
			return nil
		})
		return txErr
	})
	if err != nil {
		b.log.Warn("failed to expire Redis presence", "stream", stream, "error", err)
		return
	}

	if b.presenter != nil {
		for _, pid := range leaves {
			b.presenter.HandleLeave(stream, &common.PresenceEvent{Type: common.PresenceLeaveType, ID: pid})
		}
	}
}

func (b *RedisBroker) epochKey() string {
	return b.baseKey + ":epoch"
}

func (b *RedisBroker) sessionsMetaKey() string {
	return b.baseKey + ":sessions:meta"
}

func (b *RedisBroker) sessionKey(sid string, generation int64) string {
	return fmt.Sprintf("%s:sessions:%d:%s", b.baseKey, generation, redisKeyDigest(sid))
}

func (b *RedisBroker) historyKey(stream string) string {
	return b.baseKey + ":history:" + redisKeyDigest(stream)
}

func (b *RedisBroker) presenceSessionKey(sid string) string {
	return b.baseKey + ":presence:session:" + redisKeyDigest(sid)
}

func (b *RedisBroker) presenceIndexKey() string {
	return b.baseKey + ":presence:streams"
}

func (b *RedisBroker) presenceStreamNamesKey() string {
	return b.baseKey + ":presence:stream_names"
}

func (b *RedisBroker) presenceStreamKeys(token string) []string {
	base := b.baseKey + ":presence:" + token
	return []string{base + ":members", base + ":counts", base + ":infos", base + ":deadlines"}
}

func redisKeyDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:16])
}

func redisStreamIDToOffset(id string) (uint64, int64, error) {
	parts := strings.SplitN(id, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid Redis stream ID: %s", id)
	}
	milliseconds, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	sequence, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	if sequence > redisStreamSequenceMask {
		return 0, 0, fmt.Errorf("Redis stream sequence is too large: %d", sequence)
	}
	offset := (milliseconds << redisStreamSequenceBits) | sequence
	return offset, int64(milliseconds / 1000), nil
}

func redisOffsetToStreamID(offset uint64) string {
	milliseconds := offset >> redisStreamSequenceBits
	sequence := offset & redisStreamSequenceMask
	return fmt.Sprintf("%d-%d", milliseconds, sequence)
}
