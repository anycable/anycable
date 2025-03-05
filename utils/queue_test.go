package utils

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func testByteQueueItem(data []byte) Item[[]byte] {
	return Item[[]byte]{Data: data, Size: uint64(len(data))}
}

var initialCapacity = 2

func TestByteQueueResize(t *testing.T) {
	q := NewQueue[[]byte](initialCapacity)
	require.Equal(t, 0, q.Len())
	require.Equal(t, false, q.Closed())

	for i := 0; i < initialCapacity; i++ {
		q.Add(testByteQueueItem([]byte(strconv.Itoa(i))))
	}
	q.Add(testByteQueueItem([]byte("resize here")))
	require.Equal(t, initialCapacity*2, q.Cap())
	q.Remove()

	q.Add(testByteQueueItem([]byte("new resize here")))
	require.Equal(t, initialCapacity*2, q.Cap())
	q.Add(testByteQueueItem([]byte("one more item, no resize must happen")))
	require.Equal(t, initialCapacity*2, q.Cap())

	require.Equal(t, initialCapacity+2, q.Len())
}

func TestByteQueueSize(t *testing.T) {
	q := NewQueue[[]byte](initialCapacity)
	require.EqualValues(t, 0, q.Size())
	q.Add(testByteQueueItem([]byte("1")))
	q.Add(testByteQueueItem([]byte("2")))
	require.EqualValues(t, 2, q.Size())
	q.Remove()
	require.EqualValues(t, 1, q.Size())
}

func TestByteQueueWait(t *testing.T) {
	q := NewQueue[[]byte](initialCapacity)
	q.Add(testByteQueueItem([]byte("12")))
	q.Add(testByteQueueItem([]byte("23")))

	ok := q.Wait()
	require.Equal(t, true, ok)
	s, ok := q.Remove()
	require.Equal(t, true, ok)
	require.Equal(t, "12", string(s.Data))
	require.EqualValues(t, 2, q.Size())

	ok = q.Wait()
	require.Equal(t, true, ok)
	s, ok = q.Remove()
	require.Equal(t, true, ok)
	require.Equal(t, "23", string(s.Data))
	require.EqualValues(t, 0, q.Size())

	go func() {
		q.Add(testByteQueueItem([]byte("3")))
	}()

	ok = q.Wait()
	require.Equal(t, true, ok)
	require.EqualValues(t, 1, q.Size())
	s, ok = q.Remove()
	require.Equal(t, true, ok)
	require.Equal(t, "3", string(s.Data))
	require.EqualValues(t, 0, q.Size())
}

func TestByteQueueClose(t *testing.T) {
	q := NewQueue[[]byte](initialCapacity)

	// test removing from empty queue
	_, ok := q.Remove()
	require.Equal(t, false, ok)

	q.Add(testByteQueueItem([]byte("1")))
	q.Add(testByteQueueItem([]byte("2")))
	q.Close()

	ok = q.Add(testByteQueueItem([]byte("3")))
	require.Equal(t, false, ok)

	ok = q.Wait()
	require.Equal(t, false, ok)

	_, ok = q.Remove()
	require.Equal(t, false, ok)

	require.Equal(t, true, q.Closed())
}

func TestByteQueueClear(t *testing.T) {
	q := NewQueue[[]byte](initialCapacity)

	q.Add(testByteQueueItem([]byte("1")))
	q.Add(testByteQueueItem([]byte("2")))
	q.Clear()

	require.Equal(t, 0, q.Len())
	require.Equal(t, initialCapacity, q.Cap())
	require.EqualValues(t, 0, q.Size())
	require.Equal(t, 0, q.head)
	require.Equal(t, 0, q.tail)
	require.Equal(t, false, q.Closed())

	q.Add(testByteQueueItem([]byte("3")))

	require.Equal(t, 1, q.Len())
	require.Equal(t, initialCapacity, q.Cap())
	require.EqualValues(t, 1, q.Size())
	require.Equal(t, 0, q.head)
	require.Equal(t, 1, q.tail)
	require.Equal(t, false, q.Closed())
}
