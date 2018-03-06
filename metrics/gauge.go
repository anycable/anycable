package metrics

import (
	"sync/atomic"
)

// Gauge stores an int value
type Gauge struct {
	value int64
	Name  string
	Desc  string
}

// NewGauge initializes Gauge.
func NewGauge(name string, desc string) *Gauge {
	return &Gauge{Name: name, Desc: desc, value: 0}
}

// Set gauge value
func (g *Gauge) Set(value int) {
	atomic.StoreInt64(&g.value, int64(value))
}

// Value returns the current gauge value
func (g *Gauge) Value() int64 {
	return atomic.LoadInt64(&g.value)
}
