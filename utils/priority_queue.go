package utils

// Based on https://pkg.go.dev/container/heap

import (
	"container/heap"

	"golang.org/x/exp/constraints"
)

// An PriorityQueueItem is something we manage in a priority queue.
type PriorityQueueItem[T any, P constraints.Ordered] struct {
	value    T
	priority P
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int
}

func (pq PriorityQueueItem[T, P]) Value() T {
	return pq.value
}

func (pq PriorityQueueItem[T, P]) Priority() P {
	return pq.priority
}

type PriorityQueue[T any, P constraints.Ordered] []*PriorityQueueItem[T, P]

func NewPriorityQueue[T any, P constraints.Ordered]() *PriorityQueue[T, P] {
	pq := &PriorityQueue[T, P]{}
	heap.Init(pq)
	return pq
}

func (pq *PriorityQueue[T, P]) PushItem(v T, priority P) *PriorityQueueItem[T, P] {
	item := &PriorityQueueItem[T, P]{value: v, priority: priority}
	heap.Push(pq, item)
	return item
}

func (pq *PriorityQueue[T, P]) PopItem() *PriorityQueueItem[T, P] {
	return heap.Pop(pq).(*PriorityQueueItem[T, P])
}

// Update modifies the priority and value of an Item in the queue.
func (pq *PriorityQueue[T, P]) Update(item *PriorityQueueItem[T, P], priority P) {
	item.priority = priority
	heap.Fix(pq, item.index)
}

func (pq *PriorityQueue[T, P]) Peek() *PriorityQueueItem[T, P] {
	if len(*pq) > 0 {
		return (*pq)[0]
	}

	return nil
}

func (pq *PriorityQueue[T, P]) Remove(item *PriorityQueueItem[T, P]) {
	heap.Remove(pq, item.index)
}

func (pq PriorityQueue[T, P]) Len() int { return len(pq) }

func (pq PriorityQueue[T, P]) Less(i, j int) bool {
	return pq[i].priority < pq[j].priority
}

func (pq PriorityQueue[T, P]) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue[T, P]) Push(x any) {
	n := len(*pq)
	item := x.(*PriorityQueueItem[T, P])
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue[T, P]) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}
