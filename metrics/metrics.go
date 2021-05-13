package metrics

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/anycable/anycable-go/server"
	"github.com/apex/log"
)

const DefaultRotateInterval = 15

// IntervalHandler describe a periodical metrics writer interface
type IntervalWriter interface {
	Run(interval int) error
	Stop()
	Write(m *Metrics) error
}

// Metrics stores some useful stats about node
type Metrics struct {
	mu             sync.RWMutex
	writers        []IntervalWriter
	server         *server.HTTPServer
	httpPath       string
	rotateInterval time.Duration
	counters       map[string]*Counter
	gauges         map[string]*Gauge
	shutdownCh     chan struct{}
	log            *log.Entry
}

// FromConfig creates a new metrics instance from the prodived configuration
func FromConfig(config *Config) (*Metrics, error) {
	var metricsPrinter IntervalWriter

	writers := []IntervalWriter{}

	if config.LogEnabled() {
		if config.LogFormatterEnabled() {
			customPrinter, err := NewCustomPrinter(config.LogFormatter)

			if err == nil {
				metricsPrinter = customPrinter
			} else {
				return nil, err
			}
		} else {
			metricsPrinter = NewBasePrinter()
		}

		writers = append(writers, metricsPrinter)
	}

	instance := NewMetrics(writers, config.RotateInterval)

	if config.HTTPEnabled() {
		if config.Host != "" && config.Host != server.Host {
			srv, err := server.NewServer(config.Host, strconv.Itoa(config.Port), server.SSL)
			if err != nil {
				return nil, err
			}
			instance.server = srv
		} else {
			srv, err := server.ForPort(strconv.Itoa(config.Port))
			if err != nil {
				return nil, err
			}
			instance.server = srv
		}

		instance.httpPath = config.HTTP
		instance.server.Mux.Handle(instance.httpPath, http.HandlerFunc(instance.PrometheusHandler))
	}

	return instance, nil
}

// NewMetrics build new metrics struct
func NewMetrics(writers []IntervalWriter, rotateIntervalSeconds int) *Metrics {
	rotateInterval := time.Duration(rotateIntervalSeconds) * time.Second

	return &Metrics{
		writers:        writers,
		rotateInterval: rotateInterval,
		counters:       make(map[string]*Counter),
		gauges:         make(map[string]*Gauge),
		shutdownCh:     make(chan struct{}),
		log:            log.WithField("context", "metrics"),
	}
}

func (m *Metrics) RegisterWriter(w IntervalWriter) {
	m.writers = append(m.writers, w)
}

// Run periodically updates counters delta (and logs metrics if necessary)
func (m *Metrics) Run() error {
	if m.server != nil {
		m.log.Infof("Serve metrics at %s%s", m.server.Address(), m.httpPath)

		if err := m.server.StartAndAnnounce("Metrics server"); err != nil {
			if !m.server.Stopped() {
				return fmt.Errorf("Metrics HTTP server at %s stopped: %v", m.server.Address(), err)
			}
		}
	}

	if len(m.writers) == 0 {
		m.log.Debug("No metrics writers. Disable metrics rotation")
		return nil
	}

	if m.rotateInterval == 0 {
		m.rotateInterval = DefaultRotateInterval * time.Second
	}

	for _, writer := range m.writers {
		if err := writer.Run(int(m.rotateInterval.Seconds())); err != nil {
			return err
		}
	}

	for {
		select {
		case <-m.shutdownCh:
			return nil
		case <-time.After(m.rotateInterval):
			m.log.Debugf("Rotate metrics (interval %v)", m.rotateInterval)
			m.rotate()

			for _, writer := range m.writers {
				if err := writer.Write(m); err != nil {
					m.log.Errorf("Metrics writer failed to write: %v", err)
				}
			}
		}
	}
}

// Shutdown stops metrics updates
func (m *Metrics) Shutdown() (err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shutdownCh == nil {
		return
	}

	close(m.shutdownCh)
	m.shutdownCh = nil

	if m.server != nil {
		m.server.Shutdown() //nolint:errcheck
	}

	for _, writer := range m.writers {
		writer.Stop()
	}

	return
}

// RegisterCounter adds new counter to the registry
func (m *Metrics) RegisterCounter(name string, desc string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counters[name] = NewCounter(name, desc)
}

// RegisterGauge adds new gauge to the registry
func (m *Metrics) RegisterGauge(name string, desc string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.gauges[name] = NewGauge(name, desc)
}

// Counter returns counter by name
func (m *Metrics) Counter(name string) *Counter {
	return m.counters[name]
}

// EachCounter applies function f(*Gauge) to each gauge in a set
func (m *Metrics) EachCounter(f func(c *Counter)) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, counter := range m.counters {
		f(counter)
	}
}

// Gauge returns gauge by name
func (m *Metrics) Gauge(name string) *Gauge {
	return m.gauges[name]
}

// EachGauge applies function f(*Gauge) to each gauge in a set
func (m *Metrics) EachGauge(f func(g *Gauge)) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, gauge := range m.gauges {
		f(gauge)
	}
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
