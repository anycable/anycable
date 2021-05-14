package metrics

import "sync/atomic"

// Counter stores information about something "countable".
// Store
type Counter struct {
	name              string
	desc              string
	value             uint64
	lastIntervalValue uint64
	lastIntervalDelta uint64
}

// NewCounter creates new Counter.
func NewCounter(name string, desc string) *Counter {
	return &Counter{name: name, desc: desc, value: 0}
}

// Name returns counter name
func (c *Counter) Name() string {
	return c.name
}

// Desc returns counter description
func (c *Counter) Desc() string {
	return c.desc
}

// Value allows to get raw counter value.
func (c *Counter) Value() uint64 {
	return atomic.LoadUint64(&c.value)
}

// IntervalValue allows to get last interval value for counter.
func (c *Counter) IntervalValue() uint64 {
	if c.lastIntervalValue == 0 {
		return c.Value()
	}
	return atomic.LoadUint64(&c.lastIntervalDelta)
}

// Inc is equivalent to Add(name, 1)
func (c *Counter) Inc() uint64 {
	return c.Add(1)
}

// Add adds the given number to the counter and returns the new value.
func (c *Counter) Add(n uint64) uint64 {
	return atomic.AddUint64(&c.value, n)
}

// UpdateDelta updates the delta value for last interval based on current value and previous value.
func (c *Counter) UpdateDelta() {
	now := atomic.LoadUint64(&c.value)
	atomic.StoreUint64(&c.lastIntervalDelta, now-atomic.LoadUint64(&c.lastIntervalValue))
	atomic.StoreUint64(&c.lastIntervalValue, now)
}
