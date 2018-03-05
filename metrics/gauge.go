package metrics

import (
	"sync/atomic"
)

// Gauge stores an int value
type Gauge struct {
	value int64
}

// NewGauge initializes Gauge.
func NewGauge() *Gauge {
	return &Gauge{}
}

// Set gauge value
func (g *Gauge) Set(value int) {
	atomic.StoreInt64(&g.value, int64(value))
}

// Value returns the current gauge value
func (g *Gauge) Value() int64 {
	return atomic.LoadInt64(&g.value)
}
