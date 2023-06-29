package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPriorityQueuePushPopItem(t *testing.T) {
	pq := NewPriorityQueue[string, int]()
	b := pq.PushItem("b", 2)
	a := pq.PushItem("a", 1)
	d := pq.PushItem("d", 4)
	c := pq.PushItem("c", 3)

	assert.Equal(t, 4, len(*pq))

	assert.Same(t, a, pq.PopItem())
	assert.Same(t, b, pq.PopItem())
	assert.Same(t, c, pq.PopItem())
	assert.Same(t, d, pq.PopItem())
}

func TestPriorityQueuePeek(t *testing.T) {
	pq := NewPriorityQueue[int, int]()

	pq.PushItem(2, 2)
	pq.PushItem(1, 1)
	pq.PushItem(3, 3)

	item := pq.Peek()
	assert.Equal(t, &PriorityQueueItem[int, int]{value: 1, priority: 1}, item)
}

func TestPriorityQueueUpdate(t *testing.T) {
	pq := NewPriorityQueue[int, int]()

	item := pq.PushItem(1, 1)
	pq.PushItem(3, 3)
	pq.PushItem(5, 5)

	pq.Update(item, 4)

	one := pq.PopItem()
	assert.Equal(t, 3, one.value)

	two := pq.PopItem()
	assert.Equal(t, 1, two.value)

	three := pq.PopItem()
	assert.Equal(t, 5, three.value)
}

func TestPriorityQueueRemove(t *testing.T) {
	pq := NewPriorityQueue[int, int]()

	pq.PushItem(2, 2)
	item := pq.PushItem(1, 1)

	pq.Remove(item)

	assert.Equal(t, 1, len(*pq))
	assert.Equal(t, 2, pq.Peek().value)
}
