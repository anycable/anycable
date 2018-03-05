package metrics

import "sync/atomic"

// Counter stores information about something "countable".
// Store
type Counter struct {
	value             int64
	lastIntervalValue int64
	lastIntervalDelta int64
}

// NewCounter creates new Counter.
func NewCounter() *Counter {
	return &Counter{value: 0}
}

// Value allows to get raw counter value.
func (c *Counter) Value() int64 {
	return atomic.LoadInt64(&c.value)
}

// IntervalValue allows to get last interval value for counter.
func (c *Counter) IntervalValue() int64 {
	return atomic.LoadInt64(&c.lastIntervalDelta)
}

// Inc is equivalent to Add(name, 1)
func (c *Counter) Inc() int64 {
	return c.Add(1)
}

// Add adds the given number to the counter and returns the new value.
func (c *Counter) Add(n int64) int64 {
	return atomic.AddInt64(&c.value, n)
}

// UpdateDelta updates the delta value for last interval based on current value and previous value.
func (c *Counter) UpdateDelta() {
	now := atomic.LoadInt64(&c.value)
	atomic.StoreInt64(&c.lastIntervalDelta, now-atomic.LoadInt64(&c.lastIntervalValue))
	atomic.StoreInt64(&c.lastIntervalValue, now)
}
