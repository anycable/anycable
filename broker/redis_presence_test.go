package broker

import (
	"context"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/pubsub"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRedisBroker_Presence(t *testing.T) {
	if !redisAvailable {
		t.Skip("Redis is not available")
		return
	}

	config := NewConfig()

	broadcastHandler := FakeBroadastHandler{}
	broadcaster := pubsub.NewLegacySubscriber(broadcastHandler)

	presenter := &MockPresenter{}

	presenter.On("HandleJoin", mock.Anything, mock.Anything)
	presenter.On("HandleLeave", mock.Anything, mock.Anything)

	broker := NewRedisBroker(broadcaster, presenter, &config, &redisConfig, slog.Default())
	err := broker.Start(nil)
	require.NoError(t, err)
	defer broker.Shutdown(context.Background()) // nolint: errcheck
	require.NoError(t, broker.Reset())

	broker2 := NewRedisBroker(broadcaster, presenter, &config, &redisConfig, slog.Default())
	err = broker2.Start(nil)
	require.NoError(t, err)
	defer broker2.Shutdown(context.Background()) // nolint: errcheck

	ev, err := broker.PresenceAdd("a", "s1", "user_1", map[string]interface{}{"name": "John"})
	require.NoError(t, err)

	presenter.AssertCalled(t, "HandleJoin", "a", &common.PresenceEvent{
		Type: common.PresenceJoinType,
		ID:   "user_1",
		Info: map[string]interface{}{"name": "John"},
	})

	assert.Equal(t, "user_1", ev.ID)
	assert.Equal(t, "join", ev.Type)
	assert.Equal(t, map[string]interface{}{"name": "John"}, ev.Info)

	// Adding presence for the same session with different ID is illegal
	ev, err = broker.PresenceAdd("a", "s1", "user_2", map[string]interface{}{"name": "Boo"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "different user ID")
	assert.Nil(t, ev)

	ev, err = broker2.PresenceAdd("a", "s2", "user_1", map[string]interface{}{"name": "Jack"})
	require.NoError(t, err)
	assert.Nil(t, ev)

	// No new presence event has been triggered
	presenter.AssertNumberOfCalls(t, "HandleJoin", 1)

	ev, err = broker.PresenceAdd("a", "s3", "user_2", map[string]interface{}{"name": "Alice"})
	require.NoError(t, err)
	assert.Equal(t, "user_2", ev.ID)

	presenter.AssertNumberOfCalls(t, "HandleJoin", 2)

	ev, err = broker.PresenceAdd("b", "s3", "user_2", map[string]interface{}{"name": "Alice"})
	require.NoError(t, err)
	assert.Equal(t, "user_2", ev.ID)

	// Different stream -> new event
	presenter.AssertNumberOfCalls(t, "HandleJoin", 3)

	presenter.AssertCalled(t, "HandleJoin", "b", &common.PresenceEvent{
		Type: common.PresenceJoinType,
		ID:   "user_2",
		Info: map[string]interface{}{"name": "Alice"},
	})

	info, err := broker.PresenceInfo("a")
	require.NoError(t, err)

	assert.Equal(t, 2, info.Total)
	assert.Equal(t, 2, len(info.Records))

	// Make sure the latest info is returned
	assert.Truef(t, slices.ContainsFunc(info.Records, func(r *common.PresenceEvent) bool {
		return r.ID == "user_1" && (r.Info.(map[string]interface{})["name"] == "Jack")
	}), "presence user with user_id and name:Jack not found: %s", info.Records)

	// Now let's check that leave works
	ev, err = broker2.PresenceRemove("a", "s1")
	require.NoError(t, err)
	assert.Nil(t, ev)

	presenter.AssertNumberOfCalls(t, "HandleLeave", 0)

	info, err = broker.PresenceInfo("a")
	require.NoError(t, err)

	assert.Equal(t, 2, info.Total)

	ev, err = broker.PresenceRemove("a", "s2")
	require.NoError(t, err)
	assert.Equal(t, "user_1", ev.ID)

	presenter.AssertNumberOfCalls(t, "HandleLeave", 1)
	presenter.AssertCalled(t, "HandleLeave", "a", &common.PresenceEvent{
		Type: common.PresenceLeaveType,
		ID:   "user_1",
	})

	info, err = broker2.PresenceInfo("a")
	require.NoError(t, err)

	assert.Equal(t, 1, info.Total)
}

func TestRedisBroker_Presence_expiration(t *testing.T) {
	if !redisAvailable {
		t.Skip("Redis is not available")
		return
	}

	config := NewConfig()
	config.PresenceTTL = 1

	broadcastHandler := FakeBroadastHandler{}
	broadcaster := pubsub.NewLegacySubscriber(broadcastHandler)

	presenter := &MockPresenter{}

	presenter.On("HandleJoin", mock.Anything, mock.Anything)
	presenter.On("HandleLeave", mock.Anything, mock.Anything)

	broker := NewRedisBroker(broadcaster, presenter, &config, &redisConfig, slog.Default())
	err := broker.Start(nil)
	require.NoError(t, err)
	defer broker.Shutdown(context.Background()) // nolint: errcheck
	require.NoError(t, broker.Reset())

	broker.PresenceAdd("a", "s1", "user_1", "john") // nolint: errcheck
	broker.PresenceAdd("a", "s2", "user_2", "kate") // nolint: errcheck

	info, err := broker.PresenceInfo("a")
	require.NoError(t, err)

	assert.Equal(t, 2, info.Total)

	time.Sleep(500 * time.Millisecond)
	broker.TouchPresence("s1") // nolint: errcheck

	time.Sleep(500 * time.Millisecond)
	broker.TouchPresence("s1") // nolint: errcheck

	time.Sleep(500 * time.Millisecond)
	broker.TouchPresence("s1") // nolint: errcheck

	time.Sleep(500 * time.Millisecond)

	info, err = broker.PresenceInfo("a")
	require.NoError(t, err)

	assert.Equal(t, 1, info.Total)
	assert.Equal(t, "user_1", info.Records[0].ID)

	presenter.AssertNumberOfCalls(t, "HandleJoin", 2)
	presenter.AssertNumberOfCalls(t, "HandleLeave", 1)

	time.Sleep(1000 * time.Millisecond)

	presenter.AssertNumberOfCalls(t, "HandleLeave", 2)

	broker.PresenceAdd("a", "s3", "user_1", "jack") // nolint: errcheck

	info, err = broker.PresenceInfo("a")
	require.NoError(t, err)

	assert.Equal(t, 1, info.Total)
	assert.Equal(t, "user_1", info.Records[0].ID)
	assert.Equal(t, "jack", info.Records[0].Info)

	presenter.AssertNumberOfCalls(t, "HandleJoin", 3)
}

func TestRedisBroker_Presence_invalidation(t *testing.T) {
	if !redisAvailable {
		t.Skip("Redis is not available")
		return
	}

	config := NewConfig()
	config.PresenceTTL = 1

	broadcastHandler := FakeBroadastHandler{}
	broadcaster := pubsub.NewLegacySubscriber(broadcastHandler)

	presenter := &MockPresenter{}

	presenter.On("HandleJoin", mock.Anything, mock.Anything)
	presenter.On("HandleLeave", mock.Anything, mock.Anything)

	// l := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	l := slog.Default()

	old_broker := NewRedisBroker(broadcaster, presenter, &config, &redisConfig, l.With("test", "old"))
	err := old_broker.Start(nil)
	require.NoError(t, err)
	require.NoError(t, old_broker.Reset())

	another_broker := NewRedisBroker(broadcaster, presenter, &config, &redisConfig, l.With("test", "another"))
	err = another_broker.Start(nil)
	require.NoError(t, err)

	old_broker.PresenceAdd("c", "s1", "user_1", "john")     // nolint: errcheck
	another_broker.PresenceAdd("c", "s2", "user_2", "jane") // nolint: errcheck

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(200 * time.Millisecond):
				another_broker.TouchPresence("s2") // nolint: errcheck
			}
		}
	}()

	info, err := old_broker.PresenceInfo("c")
	require.NoError(t, err)

	assert.Equal(t, 2, info.Total)

	time.Sleep(500 * time.Millisecond)
	old_broker.TouchPresence("s1") // nolint: errcheck

	require.NoError(t, old_broker.Shutdown(context.Background()))

	presenter.AssertNumberOfCalls(t, "HandleJoin", 2)
	presenter.AssertNumberOfCalls(t, "HandleLeave", 0)

	time.Sleep(1000 * time.Millisecond)

	broker := NewRedisBroker(broadcaster, presenter, &config, &redisConfig, l.With("test", "new"))
	err = broker.Start(nil)
	require.NoError(t, err)
	defer broker.Shutdown(context.Background()) // nolint: errcheck

	time.Sleep(500 * time.Millisecond)

	info, err = broker.PresenceInfo("c")
	require.NoError(t, err)

	assert.Equal(t, 1, info.Total)

	presenter.AssertNumberOfCalls(t, "HandleLeave", 1)
}
