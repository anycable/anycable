package metrics

import (
	"sync"
	"time"

	"github.com/apex/log"
)

// Metrics stores some useful stats about node
type Metrics struct {
	mu             sync.RWMutex
	logEnabled     bool
	rotateInterval time.Duration
	counters       map[string]*Counter
	gauges         map[string]*Gauge
	shutdownCh     chan struct{}
	log            *log.Entry
}

// NewMetrics build new metrics struct
func NewMetrics(logEnabled bool, logIntervalSeconds int) *Metrics {
	rotateInterval := time.Duration(logIntervalSeconds) * time.Second

	return &Metrics{
		logEnabled:     logEnabled,
		rotateInterval: rotateInterval,
		counters:       make(map[string]*Counter),
		gauges:         make(map[string]*Gauge),
		shutdownCh:     make(chan struct{}),
		log:            log.WithField("context", "metrics"),
	}
}

// Run periodically updates counters delta (and logs metrics if necessary)
func (m *Metrics) Run() {
	if m.logEnabled {
		m.log.Infof("Log metrics every %s", m.rotateInterval)
	}

	for {
		select {
		case <-m.shutdownCh:
			return
		case <-time.After(m.rotateInterval):
			m.rotate()

			if m.logEnabled {
				snapshot := m.IntervalSnapshot()
				fields := make(log.Fields, len(snapshot))

				for k, v := range snapshot {
					fields[k] = v
				}

				m.log.WithFields(fields).Info("")
			}
		}
	}
}

// Shutdown stops metrics updates
func (m *Metrics) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shutdownCh == nil {
		return
	}

	close(m.shutdownCh)
	m.shutdownCh = nil
}

// RegisterCounter adds new counter to the registry
func (m *Metrics) RegisterCounter(name string, desc string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counters[name] = NewCounter(name, desc)
}

// RegisterGauge adds new counter to the registry
func (m *Metrics) RegisterGauge(name string, desc string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.gauges[name] = NewGauge(name, desc)
}

// Counter returns counter by name
func (m *Metrics) Counter(name string) *Counter {
	return m.counters[name]
}

// Counters returns all counters
func (m *Metrics) Counters() []Counter {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counters := make([]Counter, len(m.counters))

	i := 0

	for _, counter := range m.counters {
		dcounter := *counter
		counters[i] = dcounter
		i++
	}

	return counters
}

// Gauge returns gauge by name
func (m *Metrics) Gauge(name string) *Gauge {
	return m.gauges[name]
}

// Gauges returns all gauges
func (m *Metrics) Gauges() []Gauge {
	m.mu.RLock()
	defer m.mu.RUnlock()

	gauges := make([]Gauge, len(m.gauges))

	i := 0

	for _, gauge := range m.gauges {
		dgauge := *gauge
		gauges[i] = dgauge
		i++
	}

	return gauges
}

// IntervalSnapshot returns recorded interval metrics snapshot
func (m *Metrics) IntervalSnapshot() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := make(map[string]int64)

	for name, c := range m.counters {
		snapshot[name] = c.IntervalValue()
	}

	for name, g := range m.gauges {
		snapshot[name] = g.Value()
	}

	return snapshot
}

func (m *Metrics) rotate() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, c := range m.counters {
		c.UpdateDelta()
	}
}
