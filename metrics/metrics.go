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

// IntervalHandler describe a periodical metrics writer interface
type IntervalWriter interface {
	Run() error
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
			customPrinter, err := NewCustomPrinter(config.LogFormatter, config.LogInterval)

			if err == nil {
				metricsPrinter = customPrinter
			} else {
				return nil, err
			}
		} else {
			metricsPrinter = NewBasePrinter(config.LogInterval)
		}

		writers = append(writers, metricsPrinter)
	}

	instance := NewMetrics(writers, config.LogInterval)

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
func NewMetrics(writers []IntervalWriter, logIntervalSeconds int) *Metrics {
	rotateInterval := time.Duration(logIntervalSeconds) * time.Second

	return &Metrics{
		writers:        writers,
		rotateInterval: rotateInterval,
		counters:       make(map[string]*Counter),
		gauges:         make(map[string]*Gauge),
		shutdownCh:     make(chan struct{}),
		log:            log.WithField("context", "metrics"),
	}
}

// Run periodically updates counters delta (and logs metrics if necessary)
func (m *Metrics) Run() error {
	for _, writer := range m.writers {
		if err := writer.Run(); err != nil {
			return err
		}
	}

	if m.server != nil {
		m.log.Infof("Serve metrics at %s%s", m.server.Address(), m.httpPath)

		if err := m.server.StartAndAnnounce("Metrics server"); err != nil {
			if !m.server.Stopped() {
				return fmt.Errorf("Metrics HTTP server at %s stopped: %v", m.server.Address(), err)
			}
		}
	}

	for {
		select {
		case <-m.shutdownCh:
			return nil
		case <-time.After(m.rotateInterval):
			m.rotate()

			for _, writer := range m.writers {
				if err := writer.Write(m); err != nil {
					log.Errorf("Metrics writer failed to write: %v", err)
				}
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

	if m.server != nil {
		m.server.Stop() //nolint:errcheck
	}

	for _, writer := range m.writers {
		writer.Stop()
	}
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
