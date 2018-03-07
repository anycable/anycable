package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetricsSnapshot(t *testing.T) {
	m := NewMetrics(false, 10)

	m.RegisterCounter("test_count", "")
	m.RegisterGauge("test_gauge", "")

	for i := 0; i < 1000; i++ {
		m.Counter("test_count").Inc()
	}

	m.Gauge("test_gauge").Set(123)

	m.rotate()

	assert.Equal(t, int64(1000), m.IntervalSnapshot()["test_count"])
	assert.Equal(t, int64(123), m.IntervalSnapshot()["test_gauge"])

	m.Counter("test_count").Inc()

	m.rotate()

	assert.Equal(t, int64(1), m.IntervalSnapshot()["test_count"])
	assert.Equal(t, int64(123), m.IntervalSnapshot()["test_gauge"])
}

func TestMetricsGauges(t *testing.T) {
	m := NewMetrics(false, 10)

	m.RegisterGauge("test_gauge", "First")
	m.RegisterGauge("test_gauge_2", "Second")

	m.Gauge("test_gauge").Set(123)
	m.Gauge("test_gauge_2").Set(321)

	gauges := m.Gauges()

	for _, gauge := range gauges {
		if gauge.Name == "test_gauge" {
			assert.Equal(t, int64(123), gauge.Value())
		} else if gauge.Name == "test_gauge_2" {
			assert.Equal(t, int64(321), gauge.Value())
		} else {
			t.Errorf("Unknown gauge: %s", gauge.Name)
		}
	}

	m.Gauge("test_gauge").Set(231)

	for _, gauge := range gauges {
		if gauge.Name == "test_gauge" {
			assert.Equal(t, int64(123), gauge.Value())
		}
	}
}

func TestMetricsCounters(t *testing.T) {
	m := NewMetrics(false, 10)

	m.RegisterCounter("test_counter", "First")
	m.RegisterCounter("test_counter_2", "Second")

	m.Counter("test_counter").Inc()
	m.Counter("test_counter_2").Add(3)

	counters := m.Counters()

	for _, counter := range counters {
		if counter.Name == "test_counter" {
			assert.Equal(t, int64(1), counter.Value())
		} else if counter.Name == "test_counter_2" {
			assert.Equal(t, int64(3), counter.Value())
		} else {
			t.Errorf("Unknown counter: %s", counter.Name)
		}
	}

	m.Counter("test_counter").Inc()

	for _, counter := range counters {
		if counter.Name == "test_counter" {
			assert.Equal(t, int64(1), counter.Value())
		}
	}
}
