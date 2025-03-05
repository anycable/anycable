package utils

import (
	"sync"
)

type Item[T any] struct {
	Size uint64
	Data T
}

// Inspired by Centrifugo, which in its turn inspired by http://blog.dubbelboer.com/2015/04/25/go-faster-queue.html (MIT)
type Queue[T any] struct {
	mu      sync.RWMutex
	cond    *sync.Cond
	nodes   []Item[T]
	head    int
	tail    int
	cnt     int
	size    uint64
	closed  bool
	initCap int
}

// NewQueue Queue returns a new queue with initial capacity.
func NewQueue[T any](initialCapacity int) *Queue[T] {
	sq := &Queue[T]{
		initCap: initialCapacity,
		nodes:   make([]Item[T], initialCapacity),
	}
	sq.cond = sync.NewCond(&sq.mu)
	return sq
}

// Add an Item to the back of the queue
// will return false if the queue is closed.
// In that case the Item is dropped.
func (q *Queue[T]) Add(i Item[T]) bool {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return false
	}
	if q.cnt == len(q.nodes) {
		// Also tested a growth rate of 1.5, see: http://stackoverflow.com/questions/2269063/buffer-growth-strategy
		// In Go this resulted in a higher memory usage.
		q.resize(q.cnt * 2)
	}
	q.nodes[q.tail] = i
	q.tail = (q.tail + 1) % len(q.nodes)
	q.size += i.Size
	q.cnt++
	q.cond.Signal()
	q.mu.Unlock()
	return true
}

// Close the queue and discard all entries in the queue
// all goroutines in wait() will return
func (q *Queue[T]) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cnt = 0
	q.nodes = nil
	q.size = 0
	q.cond.Broadcast()
}

// Closed returns true if the queue has been closed
func (q *Queue[T]) Closed() bool {
	q.mu.RLock()
	c := q.closed
	q.mu.RUnlock()
	return c
}

// Clear removes all items from the queue but does not close the queue.
func (q *Queue[T]) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = false
	q.cnt = 0
	q.nodes = make([]Item[T], q.initCap)
	q.size = 0
	q.head = 0
	q.tail = 0
}

// Wait for a message to be added.
// If there are items on the queue will return immediately.
// Will return false if the queue is closed.
// Otherwise, returns true.
func (q *Queue[T]) Wait() bool {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return false
	}
	if q.cnt != 0 {
		q.mu.Unlock()
		return true
	}
	q.cond.Wait()
	q.mu.Unlock()
	return true
}

// Remove will remove an Item from the queue.
// If false is returned, it means 1) there were no items on the queue
// or 2) the queue is closed.
func (q *Queue[T]) Remove() (Item[T], bool) {
	q.mu.Lock()
	if q.cnt == 0 {
		q.mu.Unlock()
		return Item[T]{}, false
	}
	i := q.nodes[q.head]
	q.head = (q.head + 1) % len(q.nodes)
	q.cnt--
	q.size -= i.Size

	if n := len(q.nodes) / 2; n >= q.initCap && q.cnt <= n {
		q.resize(n)
	}

	q.mu.Unlock()
	return i, true
}

// Cap returns the capacity
func (q *Queue[T]) Cap() int {
	q.mu.RLock()
	c := cap(q.nodes)
	q.mu.RUnlock()
	return c
}

// Len returns the current length of the queue.
func (q *Queue[T]) Len() int {
	q.mu.RLock()
	l := q.cnt
	q.mu.RUnlock()
	return l
}

// Size returns the current size of the queue.
func (q *Queue[T]) Size() uint64 {
	q.mu.RLock()
	s := q.size
	q.mu.RUnlock()
	return s
}

func (q *Queue[T]) resize(n int) {
	nodes := make([]Item[T], n)
	if q.head < q.tail {
		copy(nodes, q.nodes[q.head:q.tail])
	} else {
		copy(nodes, q.nodes[q.head:])
		copy(nodes[len(q.nodes)-q.head:], q.nodes[:q.tail])
	}

	q.tail = q.cnt % n
	q.head = 0
	q.nodes = nodes
}
