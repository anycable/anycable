package metrics

import (
	"sync/atomic"
)

// Gauge stores an int value
type Gauge struct {
	value uint64
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
	atomic.StoreUint64(&g.value, uint64(value))
}

// Inc increment the current value by 1
func (g *Gauge) Inc() uint64 {
	return atomic.AddUint64(&g.value, 1)
}

// Dec decrement the current value by 1
func (g *Gauge) Dec() uint64 {
	return atomic.AddUint64(&g.value, ^uint64(0))
}

// Set64 sets gauge value as uint64
func (g *Gauge) Set64(value uint64) {
	atomic.StoreUint64(&g.value, value)
}

// Value returns the current gauge value
func (g *Gauge) Value() uint64 {
	return atomic.LoadUint64(&g.value)
}
