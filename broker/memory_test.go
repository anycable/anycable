package broker

import (
	"testing"
	"time"

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

	offset, err := broker.Store("test", []byte("a"), 10)
	require.NoError(t, err)
	assert.EqualValues(t, 10, offset)

	_, err = broker.Store("test", []byte("b"), 11)
	require.NoError(t, err)

	_, err = broker.Store("tes2", []byte("c"), 1)
	require.NoError(t, err)

	history, err := broker.HistoryFrom("test", broker.epoch, 10)
	require.NoError(t, err)
	assert.Len(t, history, 1)
	assert.EqualValues(t, 11, history[0].Offset)

	_, err = broker.Store("test", []byte("c"), 3)
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
