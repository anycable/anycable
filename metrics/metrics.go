package metrics

import (
	"sync"
	"time"

	"github.com/anycable/anycable-go/config"
	"github.com/apex/log"
)

const (
	updateInterval = 15 * time.Second
)

// Metrics stores some useful stats about node
type Metrics struct {
	mu         sync.RWMutex
	config     *config.Config
	snapshot   map[string]int64
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	shutdownCh chan struct{}
	log        *log.Entry
}

// NewMetrics build new metrics struct
func NewMetrics(config *config.Config) *Metrics {
	return &Metrics{
		config:     config,
		snapshot:   make(map[string]int64),
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		shutdownCh: make(chan struct{}),
		log:        log.WithField("context", "metrics"),
	}
}

// Run periodically updates metrics snapshot
func (m *Metrics) Run() {
	logMetrics := m.config.MetricsLog

	if logMetrics {
		m.log.Info("Metrics logging enabled")
	}

	for {
		select {
		case <-m.shutdownCh:
			return
		case <-time.After(updateInterval):
			m.updateSnapshot()

			if logMetrics {
				fields := make(log.Fields, len(m.snapshot))

				for k, v := range m.snapshot {
					fields[k] = v
				}

				m.log.WithFields(fields).Info("")
			}
		}
	}
}

// Shutdown stops metrics updates
func (m *Metrics) Shutdown() {
	close(m.shutdownCh)
}

// RegisterCounter adds new counter to the registry
func (m *Metrics) RegisterCounter(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counters[name] = NewCounter()
}

// RegisterGauge adds new counter to the registry
func (m *Metrics) RegisterGauge(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.gauges[name] = NewGauge()
}

// Counter returns counter by name
func (m *Metrics) Counter(name string) *Counter {
	return m.counters[name]
}

// Gauge returns gauge by name
func (m *Metrics) Gauge(name string) *Gauge {
	return m.gauges[name]
}

// Snapshot returns recorded metrics snapshot
func (m *Metrics) Snapshot() map[string]int64 {
	return m.snapshot
}

func (m *Metrics) updateSnapshot() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, c := range m.counters {
		c.UpdateDelta()
		m.snapshot[name] = c.IntervalValue()
	}

	for name, g := range m.gauges {
		m.snapshot[name] = g.Value()
	}
}
