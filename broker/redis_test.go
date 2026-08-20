package broker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	redisconfig "github.com/anycable/anycable-go/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type redisBrokerBroadcaster struct {
	mu            sync.Mutex
	broadcasts    []*common.StreamMessage
	commands      []*common.RemoteCommandMessage
	subscriptions []string
	unsubscribed  []string
}

func (b *redisBrokerBroadcaster) Broadcast(msg *common.StreamMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()

	copy := *msg
	b.broadcasts = append(b.broadcasts, &copy)
}

func (b *redisBrokerBroadcaster) BroadcastCommand(msg *common.RemoteCommandMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.commands = append(b.commands, msg)
}

func (b *redisBrokerBroadcaster) Subscribe(stream string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.subscriptions = append(b.subscriptions, stream)
}

func (b *redisBrokerBroadcaster) Unsubscribe(stream string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.unsubscribed = append(b.unsubscribed, stream)
}

type redisBrokerPresenter struct {
	mu     sync.Mutex
	joins  []*common.PresenceEvent
	leaves []*common.PresenceEvent
}

type redisBrokerUncacheable struct{}

func (redisBrokerUncacheable) ToCacheEntry() ([]byte, error) {
	return nil, errors.New("must not serialize a disabled session cache")
}

func (p *redisBrokerPresenter) HandleJoin(_ string, msg *common.PresenceEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.joins = append(p.joins, msg)
}

func (p *redisBrokerPresenter) HandleLeave(_ string, msg *common.PresenceEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.leaves = append(p.leaves, msg)
}

func testRedisBroker(t *testing.T, bro Broadcaster, presenter Presenter, config Config, prefix string) *RedisBroker {
	t.Helper()

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL is required for Redis broker integration tests")
	}

	redisConfig := redisconfig.NewRedisConfig()
	redisConfig.URL = redisURL

	instance, err := NewRedisBroker(bro, presenter, &config, &redisConfig, slog.Default(), WithRedisBrokerPrefix(prefix))
	require.NoError(t, err)
	require.NoError(t, instance.Start(nil))
	t.Cleanup(func() {
		require.NoError(t, instance.DeleteNamespace(context.Background()))
		require.NoError(t, instance.Shutdown(context.Background()))
	})

	return instance
}

func redisBrokerTestPrefix(t *testing.T) string {
	t.Helper()

	name := strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	return fmt.Sprintf("__anycable_test__:%s:%d", name, time.Now().UnixNano())
}

func TestRedisBroker_HistoryIsSharedAcrossNodes(t *testing.T) {
	config := NewConfig()
	prefix := redisBrokerTestPrefix(t)
	firstBroadcaster := &redisBrokerBroadcaster{}
	secondBroadcaster := &redisBrokerBroadcaster{}
	first := testRedisBroker(t, firstBroadcaster, nil, config, prefix)
	second := testRedisBroker(t, secondBroadcaster, nil, config, prefix)

	startedAt := time.Now().Unix()
	message := &common.StreamMessage{Stream: "reports", Data: `{"status":"ready"}`}
	require.NoError(t, first.HandleBroadcast(message))

	history, err := second.HistorySince("reports", startedAt)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, message.Data, history[0].Data)
	assert.Positive(t, history[0].Offset)
	assert.NotEmpty(t, history[0].Epoch)
	assert.Equal(t, first.Epoch(), second.Epoch())
	assert.Equal(t, first.Epoch(), history[0].Epoch)
}

func TestRedisBroker_HistoryFromOffsetAndPeak(t *testing.T) {
	config := NewConfig()
	prefix := redisBrokerTestPrefix(t)
	first := testRedisBroker(t, &redisBrokerBroadcaster{}, nil, config, prefix)
	second := testRedisBroker(t, &redisBrokerBroadcaster{}, nil, config, prefix)

	offsets := make([]uint64, 0, 3)
	for _, data := range []string{"one", "two", "three"} {
		message := &common.StreamMessage{Stream: "stream", Data: data}
		require.NoError(t, first.HandleBroadcast(message))
		offsets = append(offsets, message.Offset)
	}

	history, err := second.HistoryFrom("stream", first.Epoch(), offsets[0])
	require.NoError(t, err)
	require.Len(t, history, 2)
	assert.Equal(t, "two", history[0].Data)
	assert.EqualValues(t, offsets[1], history[0].Offset)
	assert.Equal(t, "three", history[1].Data)
	assert.EqualValues(t, offsets[2], history[1].Offset)

	peak, err := second.Peak("stream")
	require.NoError(t, err)
	require.NotNil(t, peak)
	assert.EqualValues(t, offsets[2], peak.Offset)
	assert.Equal(t, first.Epoch(), peak.Epoch)

	history, err = second.HistoryFrom("stream", "unknown", offsets[0])
	require.Error(t, err)
	assert.Nil(t, history)
}

func TestRedisBroker_HistoryLimitAndTTL(t *testing.T) {
	config := NewConfig()
	config.HistoryLimit = 2
	config.HistoryTTL = 1
	prefix := redisBrokerTestPrefix(t)
	first := testRedisBroker(t, &redisBrokerBroadcaster{}, nil, config, prefix)
	second := testRedisBroker(t, &redisBrokerBroadcaster{}, nil, config, prefix)
	startedAt := time.Now().Unix()

	for _, data := range []string{"one", "two", "three"} {
		require.NoError(t, first.HandleBroadcast(&common.StreamMessage{Stream: "stream", Data: data}))
	}

	history, err := second.HistorySince("stream", startedAt)
	require.NoError(t, err)
	require.Len(t, history, 2)
	assert.Equal(t, "two", history[0].Data)
	assert.Equal(t, "three", history[1].Data)

	require.Eventually(t, func() bool {
		history, historyErr := second.HistorySince("stream", startedAt)
		return historyErr == nil && len(history) == 0
	}, 3*time.Second, 100*time.Millisecond)
}

func TestRedisBroker_HistoryWithoutLimit(t *testing.T) {
	config := NewConfig()
	config.HistoryLimit = 0
	instance := testRedisBroker(t, &redisBrokerBroadcaster{}, nil, config, redisBrokerTestPrefix(t))
	startedAt := time.Now().Unix()

	for _, data := range []string{"one", "two", "three"} {
		require.NoError(t, instance.HandleBroadcast(&common.StreamMessage{Stream: "stream", Data: data}))
	}

	history, err := instance.HistorySince("stream", startedAt)
	require.NoError(t, err)
	require.Len(t, history, 3)
	assert.Equal(t, "one", history[0].Data)
	assert.Equal(t, "three", history[2].Data)
}

func TestRedisBroker_TransientBroadcastIsNotStored(t *testing.T) {
	config := NewConfig()
	prefix := redisBrokerTestPrefix(t)
	broadcaster := &redisBrokerBroadcaster{}
	first := testRedisBroker(t, broadcaster, nil, config, prefix)
	second := testRedisBroker(t, &redisBrokerBroadcaster{}, nil, config, prefix)

	message := &common.StreamMessage{
		Stream: "stream",
		Data:   "transient",
		Meta:   &common.StreamMessageMetadata{Transient: true},
	}
	require.NoError(t, first.HandleBroadcast(message))

	history, err := second.HistorySince("stream", time.Now().Add(-time.Minute).Unix())
	require.NoError(t, err)
	assert.Empty(t, history)
	require.Len(t, broadcaster.broadcasts, 1)
	assert.Zero(t, broadcaster.broadcasts[0].Offset)
	assert.Empty(t, broadcaster.broadcasts[0].Epoch)
}

func TestRedisBroker_SessionsAreSharedAndExpire(t *testing.T) {
	config := NewConfig()
	config.SessionsTTL = 1
	prefix := redisBrokerTestPrefix(t)
	first := testRedisBroker(t, nil, nil, config, prefix)
	second := testRedisBroker(t, nil, nil, config, prefix)

	require.NoError(t, first.CommitSession("session", &TestCacheable{"cached"}))
	restored, err := second.RestoreSession("session")
	require.NoError(t, err)
	assert.Equal(t, []byte("cached"), restored)

	time.Sleep(600 * time.Millisecond)
	require.NoError(t, second.TouchSession("session"))
	time.Sleep(600 * time.Millisecond)
	restored, err = first.RestoreSession("session")
	require.NoError(t, err)
	assert.Equal(t, []byte("cached"), restored)

	require.Eventually(t, func() bool {
		value, restoreErr := first.RestoreSession("session")
		return restoreErr == nil && value == nil
	}, 2*time.Second, 100*time.Millisecond)
}

func TestRedisBroker_SessionsDisabled(t *testing.T) {
	config := NewConfig()
	config.SessionsTTL = 0
	instance := testRedisBroker(t, nil, nil, config, redisBrokerTestPrefix(t))

	require.NoError(t, instance.CommitSession("session", redisBrokerUncacheable{}))
	restored, err := instance.RestoreSession("session")
	require.NoError(t, err)
	assert.Nil(t, restored)
	require.NoError(t, instance.TouchSession("session"))
}

func TestRedisBroker_DistributedPresenceLifecycle(t *testing.T) {
	config := NewConfig()
	prefix := redisBrokerTestPrefix(t)
	firstPresenter := &redisBrokerPresenter{}
	secondPresenter := &redisBrokerPresenter{}
	first := testRedisBroker(t, nil, firstPresenter, config, prefix)
	second := testRedisBroker(t, nil, secondPresenter, config, prefix)

	joined, err := first.PresenceAdd("room", "socket-1", "user-1", map[string]interface{}{"name": "First"})
	require.NoError(t, err)
	require.NotNil(t, joined)
	assert.Equal(t, common.PresenceJoinType, joined.Type)

	joined, err = second.PresenceAdd("room", "socket-2", "user-1", map[string]interface{}{"name": "Latest"})
	require.NoError(t, err)
	assert.Nil(t, joined)

	joined, err = second.PresenceAdd("room", "socket-3", "user-2", map[string]interface{}{"name": "Second"})
	require.NoError(t, err)
	require.NotNil(t, joined)

	info, err := first.PresenceInfo("room")
	require.NoError(t, err)
	assert.Equal(t, 2, info.Total)
	assert.ElementsMatch(t, []string{"user-1", "user-2"}, []string{info.Records[0].ID, info.Records[1].ID})

	left, err := first.PresenceRemove("room", "socket-1")
	require.NoError(t, err)
	assert.Nil(t, left)

	left, err = first.PresenceRemove("room", "socket-2")
	require.NoError(t, err)
	require.NotNil(t, left)
	assert.Equal(t, "user-1", left.ID)
	assert.Equal(t, common.PresenceLeaveType, left.Type)

	info, err = second.PresenceInfo("room")
	require.NoError(t, err)
	assert.Equal(t, 1, info.Total)
	require.Len(t, info.Records, 1)
	assert.Equal(t, "user-2", info.Records[0].ID)
}

func TestRedisBroker_PresenceTTLAndTouch(t *testing.T) {
	config := NewConfig()
	config.PresenceTTL = 1
	prefix := redisBrokerTestPrefix(t)
	first := testRedisBroker(t, nil, nil, config, prefix)
	second := testRedisBroker(t, nil, nil, config, prefix)

	_, err := first.PresenceAdd("room", "socket-1", "user-1", "kept")
	require.NoError(t, err)
	_, err = first.PresenceAdd("room", "socket-2", "user-2", "expired")
	require.NoError(t, err)

	time.Sleep(600 * time.Millisecond)
	require.NoError(t, second.TouchPresence("socket-1"))
	time.Sleep(600 * time.Millisecond)

	info, err := second.PresenceInfo("room")
	require.NoError(t, err)
	require.Equal(t, 1, info.Total)
	require.Len(t, info.Records, 1)
	assert.Equal(t, "user-1", info.Records[0].ID)
}

func TestRedisBroker_DelegatesBroadcastsCommandsAndSubscriptions(t *testing.T) {
	config := NewConfig()
	prefix := redisBrokerTestPrefix(t)
	broadcaster := &redisBrokerBroadcaster{}
	instance := testRedisBroker(t, broadcaster, nil, config, prefix)

	message := &common.StreamMessage{Stream: "stream", Data: "data"}
	require.NoError(t, instance.HandleBroadcast(message))
	require.Len(t, broadcaster.broadcasts, 1)
	assert.Positive(t, broadcaster.broadcasts[0].Offset)
	assert.NotEmpty(t, broadcaster.broadcasts[0].Epoch)

	command := &common.RemoteCommandMessage{}
	require.NoError(t, instance.HandleCommand(command))
	assert.Equal(t, []*common.RemoteCommandMessage{command}, broadcaster.commands)

	assert.Equal(t, "stream", instance.Subscribe("stream"))
	assert.Equal(t, "stream", instance.Subscribe("stream"))
	assert.Equal(t, []string{"stream"}, broadcaster.subscriptions)
	assert.Equal(t, "stream", instance.Unsubscribe("stream"))
	assert.Empty(t, broadcaster.unsubscribed)
	assert.Equal(t, "stream", instance.Unsubscribe("stream"))
	assert.Equal(t, []string{"stream"}, broadcaster.unsubscribed)
}

// The following tests mirror every TestNATSBroker_* contract in nats_test.go.

func TestRedisBroker_HistorySince_expiration(t *testing.T) {
	config := NewConfig()
	config.HistoryTTL = 1
	prefix := redisBrokerTestPrefix(t)
	instance := testRedisBroker(t, &redisBrokerBroadcaster{}, nil, config, prefix)
	instance.Subscribe("test")
	defer instance.Unsubscribe("test")

	start := time.Now().Unix() - 10
	require.NoError(t, instance.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "a"}))
	require.NoError(t, instance.HandleBroadcast(&common.StreamMessage{Stream: "test", Data: "b"}))

	time.Sleep(2 * time.Second)

	messageC := &common.StreamMessage{Stream: "test", Data: "c"}
	messageD := &common.StreamMessage{Stream: "test", Data: "d"}
	require.NoError(t, instance.HandleBroadcast(messageC))
	require.NoError(t, instance.HandleBroadcast(messageD))

	history, err := instance.HistorySince("test", start)
	require.NoError(t, err)
	require.Len(t, history, 2)
	assert.EqualValues(t, messageC.Offset, history[0].Offset)
	assert.Equal(t, "c", history[0].Data)
	assert.EqualValues(t, messageD.Offset, history[1].Offset)
	assert.Equal(t, "d", history[1].Data)

	time.Sleep(3 * time.Second)

	history, err = instance.HistorySince("test", start)
	require.NoError(t, err)
	assert.Empty(t, history)
}

func TestRedisBroker_HistorySince_with_limit(t *testing.T) {
	config := NewConfig()
	config.HistoryLimit = 2
	prefix := redisBrokerTestPrefix(t)
	instance := testRedisBroker(t, &redisBrokerBroadcaster{}, nil, config, prefix)
	instance.Subscribe("test")
	defer instance.Unsubscribe("test")

	start := time.Now().Unix() - 10
	var finalOffset uint64
	for _, data := range []string{"a", "b", "c"} {
		message := &common.StreamMessage{Stream: "test", Data: data}
		require.NoError(t, instance.HandleBroadcast(message))
		finalOffset = message.Offset
	}

	history, err := instance.HistorySince("test", start)
	require.NoError(t, err)
	assert.Len(t, history, 2)
	assert.EqualValues(t, finalOffset, history[1].Offset)
	assert.Equal(t, "c", history[1].Data)
}

func TestRedisBroker_HistoryFrom(t *testing.T) {
	config := NewConfig()
	prefix := redisBrokerTestPrefix(t)
	instance := testRedisBroker(t, &redisBrokerBroadcaster{}, nil, config, prefix)
	instance.Subscribe("test")
	defer instance.Unsubscribe("test")

	offsets := make([]uint64, 0, 7)
	for _, data := range []string{"y", "z", "a", "b", "c", "d", "e"} {
		message := &common.StreamMessage{Stream: "test", Data: data}
		require.NoError(t, instance.HandleBroadcast(message))
		offsets = append(offsets, message.Offset)
	}

	t.Run("With current epoch", func(t *testing.T) {
		history, err := instance.HistoryFrom("test", instance.Epoch(), offsets[3])
		require.NoError(t, err)
		require.Len(t, history, 3)
		assert.EqualValues(t, offsets[4], history[0].Offset)
		assert.Equal(t, "c", history[0].Data)
		assert.EqualValues(t, offsets[5], history[1].Offset)
		assert.Equal(t, "d", history[1].Data)
		assert.EqualValues(t, offsets[6], history[2].Offset)
		assert.Equal(t, "e", history[2].Data)
	})

	t.Run("When no new messages", func(t *testing.T) {
		history, err := instance.HistoryFrom("test", instance.Epoch(), offsets[6])
		require.NoError(t, err)
		assert.Empty(t, history)
	})

	t.Run("When no stream", func(t *testing.T) {
		history, err := instance.HistoryFrom("unknown", instance.Epoch(), offsets[3])
		require.Error(t, err)
		assert.Nil(t, history)
	})

	t.Run("With unknown epoch", func(t *testing.T) {
		history, err := instance.HistoryFrom("test", "unknown", offsets[3])
		require.Error(t, err)
		assert.Nil(t, history)
	})
}

func TestRedisBroker_Sessions(t *testing.T) {
	config := NewConfig()
	config.SessionsTTL = 1
	prefix := redisBrokerTestPrefix(t)
	first := testRedisBroker(t, nil, nil, config, prefix)

	require.NoError(t, first.CommitSession("test123", &TestCacheable{"cache-me"}))

	second := testRedisBroker(t, nil, nil, config, prefix)
	restored, err := second.RestoreSession("test123")
	require.NoError(t, err)
	assert.Equal(t, []byte("cache-me"), restored)

	time.Sleep(2 * time.Second)
	expired, err := first.RestoreSession("test123")
	require.NoError(t, err)
	assert.Nil(t, expired)

	require.NoError(t, first.CommitSession("test345", &TestCacheable{"cache-me-again"}))
	committed, err := second.RestoreSession("test345")
	require.NoError(t, err)
	assert.Equal(t, []byte("cache-me-again"), committed)

	time.Sleep(500 * time.Millisecond)
	require.NoError(t, first.TouchSession("test345"))
	time.Sleep(500 * time.Millisecond)

	finished, err := second.RestoreSession("test345")
	require.NoError(t, err)
	assert.Equal(t, []byte("cache-me-again"), finished)

	time.Sleep(1 * time.Second)
	finishedStale, err := second.RestoreSession("test345")
	require.NoError(t, err)
	assert.Nil(t, finishedStale)
}

func TestRedisBroker_SessionsTTLChange(t *testing.T) {
	config := NewConfig()
	config.SessionsTTL = 1
	prefix := redisBrokerTestPrefix(t)
	first := testRedisBroker(t, nil, nil, config, prefix)
	require.NoError(t, first.CommitSession("test123", &TestCacheable{"cache-me"}))

	otherConfig := NewConfig()
	otherConfig.SessionsTTL = 3
	second := testRedisBroker(t, nil, nil, otherConfig, prefix)

	missing, err := second.RestoreSession("test123")
	require.NoError(t, err)
	assert.Nil(t, missing)

	require.NoError(t, second.CommitSession("test234", &TestCacheable{"cache-me-again"}))
	time.Sleep(1 * time.Second)

	restored, err := first.RestoreSession("test234")
	require.NoError(t, err)
	assert.Equal(t, []byte("cache-me-again"), restored)

	require.NoError(t, second.TouchSession("test234"))
	time.Sleep(2 * time.Second)

	restoredAgain, err := first.RestoreSession("test234")
	require.NoError(t, err)
	assert.NotNil(t, restoredAgain)

	time.Sleep(2 * time.Second)
	expired, err := first.RestoreSession("test234")
	require.NoError(t, err)
	assert.Nil(t, expired)
}

func TestRedisBroker_Epoch(t *testing.T) {
	config := NewConfig()
	prefix := redisBrokerTestPrefix(t)
	first := testRedisBroker(t, nil, nil, config, prefix)
	epoch := first.Epoch()

	second := testRedisBroker(t, nil, nil, config, prefix)
	assert.Equal(t, epoch, second.Epoch())

	require.NoError(t, second.SetEpoch("new-epoch"))
	assert.Equal(t, "new-epoch", second.Epoch())
	require.Eventually(t, func() bool {
		return first.Epoch() == "new-epoch"
	}, 2*time.Second, 100*time.Millisecond)
}
