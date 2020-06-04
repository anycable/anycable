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

// Printer describes metrics logging interface
type Printer interface {
	Print(snapshot map[string]int64)
}

// Metrics stores some useful stats about node
type Metrics struct {
	mu             sync.RWMutex
	printer        Printer
	server         *server.HTTPServer
	httpPath       string
	rotateInterval time.Duration
	counters       map[string]*Counter
	gauges         map[string]*Gauge
	shutdownCh     chan struct{}
	log            *log.Entry
}

// BasePrinter simply logs stats as structured log
type BasePrinter struct {
}

// NewBasePrinter returns new base printer struct
func NewBasePrinter() *BasePrinter {
	return &BasePrinter{}
}

// Print logs stats data using global logger with info level
func (*BasePrinter) Print(snapshot map[string]int64) {
	fields := make(log.Fields, len(snapshot)+1)

	fields["context"] = "metrics"

	for k, v := range snapshot {
		fields[k] = v
	}

	log.WithFields(fields).Info("")
}

// FromConfig creates a new metrics instance from the prodived configuration
func FromConfig(config *Config) (*Metrics, error) {
	var metricsPrinter Printer

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
	}

	instance := NewMetrics(metricsPrinter, config.LogInterval)

	if config.HTTPEnabled() {
		server, err := server.ForPort(strconv.Itoa(config.Port))
		if err != nil {
			return nil, err
		}
		instance.server = server
		instance.httpPath = config.HTTP
		instance.server.Mux.Handle(instance.httpPath, http.HandlerFunc(instance.PrometheusHandler))
	}

	return instance, nil
}

// NewMetrics build new metrics struct
func NewMetrics(printer Printer, logIntervalSeconds int) *Metrics {
	rotateInterval := time.Duration(logIntervalSeconds) * time.Second

	return &Metrics{
		printer:        printer,
		rotateInterval: rotateInterval,
		counters:       make(map[string]*Counter),
		gauges:         make(map[string]*Gauge),
		shutdownCh:     make(chan struct{}),
		log:            log.WithField("context", "metrics"),
	}
}

// Run periodically updates counters delta (and logs metrics if necessary)
func (m *Metrics) Run() error {
	if m.printer != nil {
		m.log.Infof("Log metrics every %s", m.rotateInterval)
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

			if m.printer != nil {
				snapshot := m.IntervalSnapshot()

				m.printer.Print(snapshot)
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
