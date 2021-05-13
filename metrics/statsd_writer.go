package metrics

import (
	"strings"
	"sync"

	"github.com/apex/log"
	"github.com/smira/go-statsd"
)

type StatsdConfig struct {
	Host          string
	Prefix        string
	MaxPacketSize int
}

type StatsdLogger struct {
	log *log.Entry
}

func (lg *StatsdLogger) Printf(msg string, args ...interface{}) {
	msg = strings.TrimPrefix(msg, "[STATSD] ")
	// Statsd only prints errors and warnings
	if strings.Contains(msg, "Error") {
		lg.log.Errorf(msg, args...)
	} else {
		lg.log.Warnf(msg, args...)
	}
}

func NewStatsdConfig() StatsdConfig {
	return StatsdConfig{Prefix: "anycable_go.", MaxPacketSize: 1400}
}

func (c StatsdConfig) Enabled() bool {
	return c.Host != ""
}

type StatsdWriter struct {
	client *statsd.Client
	config StatsdConfig

	mu sync.Mutex
}

var _ IntervalWriter = (*StatsdWriter)(nil)

func NewStatsdWriter(c StatsdConfig) *StatsdWriter {
	return &StatsdWriter{config: c}
}

func (sw *StatsdWriter) Run(interval int) error {
	sl := StatsdLogger{log.WithField("context", "statsd")}
	sw.client = statsd.NewClient(
		sw.config.Host,
		statsd.MaxPacketSize(sw.config.MaxPacketSize),
		statsd.MetricPrefix(sw.config.Prefix),
		statsd.Logger(&sl),
	)

	log.WithField("context", "metrics").Infof("Send statsd metrics to %s with prefix %s every %vs", sw.config.Host, sw.config.Prefix, interval)

	return nil
}

func (sw *StatsdWriter) Stop() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.client.Close()
	sw.client = nil
}

func (sw *StatsdWriter) Write(m *Metrics) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.client == nil {
		return nil
	}

	m.EachCounter(func(counter *Counter) {
		sw.client.Incr(counter.Name(), int64(counter.IntervalValue()))
	})

	m.EachGauge(func(gauge *Gauge) {
		sw.client.Gauge(gauge.Name(), int64(gauge.Value()))
	})

	return nil
}
