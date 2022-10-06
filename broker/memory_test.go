package broker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpire(t *testing.T) {
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
	assert.Nil(t, history)
}

func TestLimit(t *testing.T) {
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

func TestFromOffset(t *testing.T) {
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
