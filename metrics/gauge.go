package metrics

import (
	"sync/atomic"
)

// Gauge stores an int value
type Gauge struct {
	value int64
	name  string
	desc  string
}

// NewGauge initializes Gauge.
func NewGauge(name string, desc string) *Gauge {
	return &Gauge{name: name, desc: desc, value: 0}
}

// Name returns gauge name
func (g *Gauge) Name() string {
	return g.name
}

// Desc returns gauge description
func (g *Gauge) Desc() string {
	return g.desc
}

// Set gauge value
func (g *Gauge) Set(value int) {
	atomic.StoreInt64(&g.value, int64(value))
}

// Inc increment the current value by 1
func (g *Gauge) Inc() int64 {
	return atomic.AddInt64(&g.value, 1)
}

// Dec decrement the current value by 1
func (g *Gauge) Dec() int64 {
	return atomic.AddInt64(&g.value, -1)
}

// Set64 sets gauge value as int64
func (g *Gauge) Set64(value int64) {
	atomic.StoreInt64(&g.value, value)
}

// Value returns the current gauge value
func (g *Gauge) Value() int64 {
	return atomic.LoadInt64(&g.value)
}
