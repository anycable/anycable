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

// Value returns the current gauge value
func (g *Gauge) Value() int64 {
	return atomic.LoadInt64(&g.value)
}
