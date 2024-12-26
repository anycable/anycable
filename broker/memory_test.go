package broker

import (
	"slices"
	"testing"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemory_Expire(t *testing.T) {
	config := NewConfig()
	config.HistoryTTL = 1

	broker := NewMemoryBroker(nil, &config)

	start := time.Now().Unix() - 10

	broker.add("test", "a")
	broker.add("test", "b")

	time.Sleep(2 * time.Second)

	broker.add("test", "c")
	broker.add("test", "d")

	broker.expire()

	history, err := broker.HistorySince("test", start)
	require.NoError(t, err)

	assert.Len(t, history, 2)
	assert.EqualValues(t, 3, history[0].Offset)
	assert.Equal(t, "c", history[0].Data)
	assert.EqualValues(t, 4, history[1].Offset)
	assert.Equal(t, "d", history[1].Data)

	time.Sleep(2 * time.Second)

	broker.expire()

	history, err = broker.HistorySince("test", start)
	require.NoError(t, err)
	assert.Empty(t, history)
}

func TestMemory_Limit(t *testing.T) {
	config := NewConfig()
	config.HistoryLimit = 2

	broker := NewMemoryBroker(nil, &config)

	start := time.Now().Unix() - 10

	broker.add("test", "a")
	broker.add("test", "b")
	broker.add("test", "c")

	history, err := broker.HistorySince("test", start)
	require.NoError(t, err)

	assert.Len(t, history, 2)
	assert.EqualValues(t, 3, history[1].Offset)
	assert.Equal(t, "c", history[1].Data)
}

func TestMemory_FromOffset(t *testing.T) {
	config := NewConfig()

	broker := NewMemoryBroker(nil, &config)

	broker.add("test", "a")
	broker.add("test", "b")
	broker.add("test", "c")
	broker.add("test", "d")
	broker.add("test", "e")

	t.Run("With current epoch", func(t *testing.T) {
		history, err := broker.HistoryFrom("test", broker.epoch, 2)
		require.NoError(t, err)

		assert.Len(t, history, 3)
		assert.EqualValues(t, 3, history[0].Offset)
		assert.Equal(t, "c", history[0].Data)
		assert.EqualValues(t, 4, history[1].Offset)
		assert.Equal(t, "d", history[1].Data)
		assert.EqualValues(t, 5, history[2].Offset)
		assert.Equal(t, "e", history[2].Data)
	})

	t.Run("With unknown epoch", func(t *testing.T) {
		history, err := broker.HistoryFrom("test", "unknown", 2)
		require.Error(t, err)
		require.Nil(t, history)
	})
}

func TestMemory_Store(t *testing.T) {
	config := NewConfig()

	broker := NewMemoryBroker(nil, &config)
	broker.SetEpoch("2023")

	ts := time.Now()

	offset, err := broker.Store("test", []byte("a"), 10, ts)
	require.NoError(t, err)
	assert.EqualValues(t, 10, offset)

	_, err = broker.Store("test", []byte("b"), 11, ts)
	require.NoError(t, err)

	_, err = broker.Store("tes2", []byte("c"), 1, ts)
	require.NoError(t, err)

	history, err := broker.HistoryFrom("test", broker.epoch, 10)
	require.NoError(t, err)
	assert.Len(t, history, 1)
	assert.EqualValues(t, 11, history[0].Offset)

	_, err = broker.Store("test", []byte("c"), 3, ts)
	assert.Error(t, err)
}

func TestMemstream_filterByOffset(t *testing.T) {
	ms := &memstream{
		ttl:   1,
		limit: 5,
	}

	ms.add("test1")
	ms.add("test2")

	// Should return error if offset is out of range
	err := ms.filterByOffset(10, func(e *entry) {})
	require.Error(t, err)

	err = ms.filterByOffset(1, func(e *entry) {
		assert.Equal(t, "test2", e.data)
	})
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	ms.expire()

	err = ms.filterByOffset(1, func(e *entry) {
		assert.Failf(t, "entry should be expired", "entry: %v", e)
	})
	require.Error(t, err)
}

func TestMemory_Presence(t *testing.T) {
	config := NewConfig()

	broker := NewMemoryBroker(nil, &config)

	ev, err := broker.PresenceAdd("a", "s1", "user_1", map[string]interface{}{"name": "John"})
	require.NoError(t, err)

	assert.Equal(t, "user_1", ev.ID)
	assert.Equal(t, "join", ev.Type)
	assert.Equal(t, map[string]interface{}{"name": "John"}, ev.Info)

	// Adding presence for the same session with different ID is illegal
	ev, err = broker.PresenceAdd("a", "s1", "user_2", map[string]interface{}{"name": "Boo"})
	require.Error(t, err)
	assert.Nil(t, ev)

	ev, err = broker.PresenceAdd("a", "s2", "user_1", map[string]interface{}{"name": "Jack"})
	require.NoError(t, err)
	assert.Nil(t, ev)

	ev, err = broker.PresenceAdd("a", "s3", "user_2", map[string]interface{}{"name": "Alice"})
	require.NoError(t, err)
	assert.Equal(t, "user_2", ev.ID)

	ev, err = broker.PresenceAdd("b", "s3", "user_2", map[string]interface{}{"name": "Alice"})
	require.NoError(t, err)
	assert.Equal(t, "user_2", ev.ID)

	info, err := broker.PresenceInfo("a")
	require.NoError(t, err)

	assert.Equal(t, 2, info.Total)
	assert.Equal(t, 2, len(info.Records))

	// Make sure the latest info is returned
	assert.Truef(t, slices.ContainsFunc(info.Records, func(r *common.PresenceEvent) bool {
		return r.ID == "user_1" && (r.Info.(map[string]interface{})["name"] == "Jack")
	}), "presence user with user_id and name:Jack not found: %s", info.Records)

	// Now let's check that leave works
	ev, err = broker.PresenceRemove("a", "s1")
	require.NoError(t, err)
	assert.Nil(t, ev)

	info, err = broker.PresenceInfo("a")
	require.NoError(t, err)

	assert.Equal(t, 2, info.Total)

	ev, err = broker.PresenceRemove("a", "s2")
	require.NoError(t, err)
	assert.Equal(t, "user_1", ev.ID)

	info, err = broker.PresenceInfo("a")
	require.NoError(t, err)

	assert.Equal(t, 1, info.Total)
}

func TestMemory_expirePresence(t *testing.T) {
	config := NewConfig()
	config.PresenceTTL = 1

	broker := NewMemoryBroker(nil, &config)

	broker.PresenceAdd("a", "s1", "user_1", "john") // nolint: errcheck
	broker.PresenceAdd("a", "s2", "user_2", "kate") // nolint: errcheck

	info, err := broker.PresenceInfo("a")
	require.NoError(t, err)

	assert.Equal(t, 2, info.Total)

	broker.FinishPresence("s1") // nolint: errcheck
	broker.FinishPresence("s2") // nolint: errcheck

	time.Sleep(2 * time.Second)

	broker.PresenceAdd("a", "s3", "user_1", "john") // nolint: errcheck

	broker.expire()

	info, err = broker.PresenceInfo("a")
	require.NoError(t, err)

	assert.Equal(t, 1, info.Total)
	assert.Equal(t, "user_1", info.Records[0].ID)
}
