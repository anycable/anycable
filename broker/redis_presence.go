package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "embed"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/redis"
	"github.com/anycable/anycable-go/utils"
	"github.com/joomcode/errorx"
	"github.com/redis/rueidis"
)

const (
	redisPresencePrefix        = "$ac:p:"
	redisUserPresencePrefix    = "$ac:pu:"
	redisSessionPresencePrefix = "$ac:ps:"
)

func (b *RedisBroker) SupportsPresence() bool {
	return true
}

func (b *RedisBroker) PresenceAdd(stream string, sid string, pid string, info interface{}) (*common.PresenceEvent, error) {
	client, err := b.Client()

	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	key := stream
	ttl := fmt.Sprintf("%d", b.conf.PresenceTTL)
	infoJSON := string(utils.ToJSON(info))

	res := b.presence.addPresenceScript.Exec(ctx, client, []string{key}, []string{sid, pid, ttl, infoJSON})

	if res.Error() != nil {
		return nil, errorx.Decorate(res.Error(), "failed to store presence in Redis")
	}

	b.presence.add(sid, stream, pid)

	newPresence, err := res.ToInt64()

	if err != nil {
		return nil, errorx.Decorate(err, "failed to parse presence result from Redis response")
	}

	if newPresence > 0 {
		ev := &common.PresenceEvent{
			Type: common.PresenceJoinType,
			ID:   pid,
			Info: info,
		}

		if b.presenter != nil {
			b.presenter.HandleJoin(stream, ev)
		}

		return ev, nil
	}

	return nil, nil
}

func (b *RedisBroker) PresenceRemove(stream string, sid string) (*common.PresenceEvent, error) {
	client, err := b.Client()

	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	key := stream

	res := b.presence.removePresenceScript.Exec(ctx, client, []string{key}, []string{sid})

	if res.Error() != nil {
		return nil, errorx.Decorate(res.Error(), "failed to remove presence from Redis")
	}

	b.presence.remove(sid, stream)

	pid, err := res.ToString()

	if err != nil {
		return nil, errorx.Decorate(err, "failed to parse presence result from Redis response")
	}

	if pid != "" {
		ev := &common.PresenceEvent{
			Type: common.PresenceLeaveType,
			ID:   pid,
		}

		if b.presenter != nil {
			b.presenter.HandleLeave(stream, ev)
		}

		return ev, nil
	}

	return nil, nil
}

func (b *RedisBroker) PresenceInfo(stream string, opts ...PresenceInfoOption) (*common.PresenceInfo, error) {
	client, err := b.Client()

	if err != nil {
		return nil, err
	}

	options := NewPresenceInfoOptions()
	for _, opt := range opts {
		opt(options)
	}

	info := common.NewPresenceInfo()

	ctx := context.Background()
	key := redisPresencePrefix + stream

	if !b.keyExists(ctx, key) {
		return info, nil
	}

	if !options.ReturnRecords {
		res := client.Do(ctx, client.B().Hlen().Key(key).Build())

		size, err := res.ToInt64()

		if err != nil {
			return nil, errorx.Decorate(err, "failed to parse presence size from Redis response")
		}

		info.Total = int(size)
		return info, nil
	} else {
		res := client.Do(ctx, client.B().Hgetall().Key(key).Build())

		if res.Error() != nil {
			return nil, errorx.Decorate(res.Error(), "failed to get presence from Redis")
		}

		values, err := res.AsStrMap()

		if err != nil {
			return nil, errorx.Decorate(err, "failed to parse presence from Redis response")
		}

		info.Records = make([]*common.PresenceEvent, 0, len(values))

		for id, v := range values {
			var userInfo interface{}
			err := json.Unmarshal([]byte(v), &userInfo)
			if err != nil {
				return nil, errorx.Decorate(err, "failed to parse user presence info")
			}

			info.Records = append(info.Records, &common.PresenceEvent{
				Info: userInfo,
				ID:   id,
			})
		}

		info.Total = len(info.Records)
		return info, nil
	}
}

func (b *RedisBroker) TouchPresence(sid string) error {
	client, err := b.Client()

	if err != nil {
		return err
	}

	ctx := context.Background()
	key := sid
	ttl := fmt.Sprintf("%d", b.conf.PresenceTTL)

	res := b.presence.touchPresenceScript.Exec(ctx, client, []string{key}, []string{ttl})

	if res.Error() != nil {
		return errorx.Decorate(res.Error(), "failed to store presence in Redis")
	}

	b.presence.touch(sid)

	return nil
}

func (b *RedisBroker) MaybeExpirePresence(sid string, stream string, pid string) error {
	client, err := b.Client()

	if err != nil {
		return err
	}

	ctx := context.Background()
	key := stream

	b.log.Debug("check presence expiration", "sid", sid, "pid", pid, "stream", stream)

	res := b.presence.expirePresenceScript.Exec(ctx, client, []string{key}, []string{sid, pid})

	if res.Error() != nil {
		return errorx.Decorate(res.Error(), "failed to store presence in Redis")
	}

	lastPresence, err := res.ToInt64()

	if err != nil {
		return errorx.Decorate(err, "failed to parse presence result from Redis response")
	}

	if lastPresence > 0 {
		b.log.Debug("presence expired", "sid", sid, "pid", pid, "stream", stream)

		ev := &common.PresenceEvent{
			Type: common.PresenceLeaveType,
			ID:   pid,
		}

		if b.presenter != nil {
			b.presenter.HandleLeave(stream, ev)
		}

		return nil
	}

	return nil
}

// The presence is stored in Redis as follows:
//   - $ac:p:<stream>: Hash<pid,info> (info_ttl)
//   - $ac:pu:<stream>:<pid>: Hash<sid, ''> (ttl)
//   - $ac:ps:<sid>: Hash<stream, pid> (cleanup_ttl)
//
//  Where:
//    - info_ttl = ttl * 3
//    - cleanup_ttl = ttl * 10

//go:embed redis/add_presence.lua
var addPresenceSource string

//go:embed redis/touch_presence.lua
var touchPresenceSource string

//go:embed redis/remove_presence.lua
var removePresenceSource string

//go:embed redis/expire_presence.lua
var expirePresenceSource string

//go:embed redis/invalidate_presence.lua
var invalidatePresenceSource string

const invalidationScanStep = 100
const invalidationCooldownMS = 100
const invalidationStream = "__anycable_presence__"
const invalidationConsumerGroup = "px"
const invalidationReadBlockMS = 1000

type redisPresence struct {
	bro *RedisBroker

	addPresenceScript        *rueidis.Lua
	touchPresenceScript      *rueidis.Lua
	removePresenceScript     *rueidis.Lua
	expirePresenceScript     *rueidis.Lua
	invalidatePresenceScript *rueidis.Lua

	sessions map[string]*presenceSessionEntry
	smu      sync.Mutex

	// Used to invalidate presence information on startup
	invalidateConsumer *redis.Streamer

	stopCtx  context.Context
	cancelFn context.CancelFunc
}

func NewRedisPresence(b *RedisBroker) *redisPresence {
	ctx, cancel := context.WithCancel(context.Background())

	rp := &redisPresence{
		bro: b,

		addPresenceScript:        rueidis.NewLuaScript(addPresenceSource),
		touchPresenceScript:      rueidis.NewLuaScript(touchPresenceSource),
		removePresenceScript:     rueidis.NewLuaScript(removePresenceSource),
		expirePresenceScript:     rueidis.NewLuaScript(expirePresenceSource),
		invalidatePresenceScript: rueidis.NewLuaScript(invalidatePresenceSource),

		sessions: make(map[string]*presenceSessionEntry),

		stopCtx:  ctx,
		cancelFn: cancel,
	}

	ic := redis.NewStreamer(
		invalidationStream,
		invalidationConsumerGroup,
		b.rconf,
		b.log.With("component", "invalidator"),
		redis.StreamerWithHandler(rp.handleInvalidateTask),
		redis.StreamerWithBlockMS(invalidationReadBlockMS),
	)

	rp.invalidateConsumer = ic

	return rp
}

func (rp *redisPresence) Start() error {
	go rp.expireLoop()
	if err := rp.invalidateConsumer.Start(); err != nil {
		return errorx.Decorate(err, "failed to start invalidation stream consumer")
	}
	go rp.invalidationLoop()
	return nil
}

func (rp *redisPresence) Stop(ctx context.Context) error {
	rp.cancelFn()
	if err := rp.invalidateConsumer.Shutdown(ctx); err != nil {
		return errorx.Decorate(err, "failed to stop invalidation stream consumer")
	}
	return nil
}

func (rp *redisPresence) handleInvalidateTask(fields map[string]string) error {
	var sid string
	var pid string
	var stream string

	if val, ok := fields["stream"]; ok {
		stream = val
	} else {
		return fmt.Errorf("stream is missing: %v", fields)
	}

	if prefixedVal, ok := fields["key"]; ok {
		if val, ok := strings.CutPrefix(prefixedVal, redisSessionPresencePrefix); ok {
			sid = val
		} else {
			return fmt.Errorf("invalid key format: %s", prefixedVal)
		}
	} else {
		return fmt.Errorf("key is missing: %v", fields)
	}

	if val, ok := fields["user_id"]; ok {
		pid = val
	} else {
		return fmt.Errorf("user_id is missing: %v", fields)
	}

	rp.bro.MaybeExpirePresence(sid, stream, pid) // nolint:errcheck

	return nil
}

func (rp *redisPresence) invalidationLoop() {
	nextCursor := "0"

	for {
		select {
		case <-rp.stopCtx.Done():
			return
		case <-time.After(invalidationCooldownMS * time.Millisecond):
			client, err := rp.bro.Client()
			if err != nil {
				rp.bro.log.Debug("failed to get redis client", "error", err)
				continue
			}

			ctx := context.Background()

			// expiration for an alive session must be between 2 and 3 ttl (since we touch it with 3*ttl)
			deadline := time.Now().UnixMilli() + (rp.bro.conf.PresenceTTL * 9000)
			deadlineStr := fmt.Sprintf("%d", deadline)

			rp.bro.log.Debug("invalidate sessions", "deadline", deadline, "cursor", nextCursor)

			res := rp.invalidatePresenceScript.Exec(ctx, client, []string{invalidationStream}, []string{nextCursor, fmt.Sprintf("%d", invalidationScanStep), deadlineStr})

			if res.Error() != nil {
				rp.bro.log.Error("failed to invalidate presence", "error", res.Error())
				return
			}

			nextStep, err := res.ToArray()

			if err != nil {
				rp.bro.log.Error("failed to parse invalidation result", "error", res.Error())
				return
			}

			nextCursor, err = nextStep[0].ToString()

			if err != nil {
				rp.bro.log.Error("failed to parse next cursor", "error", res.Error())
				return
			}

			expiredCount, err := nextStep[1].ToInt64()

			if err != nil {
				rp.bro.log.Error("failed to parse expired count", "error", err)
				return
			}

			if nextCursor == "0" {
				rp.bro.log.Debug("sessions invalidation completed", "count", expiredCount)
				return
			} else {
				rp.bro.log.Debug("sessions invalidation in progress", "count", expiredCount)
			}
		}
	}
}

func (rp *redisPresence) add(sid string, stream string, pid string) {
	rp.smu.Lock()
	defer rp.smu.Unlock()

	if _, ok := rp.sessions[sid]; !ok {
		rp.sessions[sid] = &presenceSessionEntry{
			streams:  make(map[string]string),
			deadline: time.Now().UnixMilli() + (rp.bro.conf.PresenceTTL * 1000),
		}
	}

	rp.sessions[sid].streams[stream] = pid
}

func (rp *redisPresence) touch(sid string) {
	rp.smu.Lock()
	defer rp.smu.Unlock()

	if _, ok := rp.sessions[sid]; !ok {
		return
	}

	deadline := time.Now().UnixMilli() + (rp.bro.conf.PresenceTTL * 1000)

	rp.sessions[sid].deadline = deadline
}

func (rp *redisPresence) remove(sid string, stream string) {
	rp.smu.Lock()
	defer rp.smu.Unlock()

	if _, ok := rp.sessions[sid]; !ok {
		return
	}

	delete(rp.sessions[sid].streams, stream)

	if len(rp.sessions[sid].streams) == 0 {
		delete(rp.sessions, sid)
	}
}

func (rp *redisPresence) expireLoop() {
	for {
		select {
		case <-rp.stopCtx.Done():
			return
		case <-time.After(time.Second):
			rp.expire()
		}
	}
}

type expiredPresence struct {
	sid    string
	pid    string
	stream string
}

func (rp *redisPresence) expire() {
	rp.smu.Lock()

	now := time.Now().UnixMilli()
	toDelete := make([]*expiredPresence, 0)

	for sid, sp := range rp.sessions {
		if sp.deadline > 0 && sp.deadline < now {
			for st, pid := range sp.streams {
				toDelete = append(toDelete, &expiredPresence{sid: sid, pid: pid, stream: st})
			}
		}
	}

	for _, ep := range toDelete {
		delete(rp.sessions, ep.sid)
	}

	rp.smu.Unlock()

	for _, ep := range toDelete {
		err := rp.bro.MaybeExpirePresence(ep.sid, ep.stream, ep.pid)
		if err != nil {
			rp.bro.log.Warn("failed to expire presence info", "sid", ep.sid, "pid", ep.pid, "stream", ep.stream, "error", err)
		}
	}
}
