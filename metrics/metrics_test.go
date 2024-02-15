package metrics

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetricsSnapshot(t *testing.T) {
	m := NewMetrics(nil, 10, slog.Default())

	m.RegisterCounter("test_count", "")
	m.RegisterGauge("test_gauge", "")

	for i := 0; i < 1000; i++ {
		m.Counter("test_count").Inc()
	}

	m.Gauge("test_gauge").Set(123)

	m.rotate()

	assert.Equal(t, uint64(1000), m.IntervalSnapshot()["test_count"])
	assert.Equal(t, uint64(123), m.IntervalSnapshot()["test_gauge"])

	m.Counter("test_count").Inc()

	m.rotate()

	assert.Equal(t, uint64(1), m.IntervalSnapshot()["test_count"])
	assert.Equal(t, uint64(123), m.IntervalSnapshot()["test_gauge"])
}

func TestMetrics_EachGauge(t *testing.T) {
	m := NewMetrics(nil, 10, slog.Default())

	m.RegisterGauge("test_gauge", "First")
	m.RegisterGauge("test_gauge_2", "Second")

	m.Gauge("test_gauge").Set(123)
	m.Gauge("test_gauge_2").Set(321)

	m.EachGauge(func(gauge *Gauge) {
		switch gauge.Name() {
		case "test_gauge":
			{
				assert.Equal(t, uint64(123), gauge.Value())
			}
		case "test_gauge_2":
			{
				assert.Equal(t, uint64(321), gauge.Value())
			}
		default:
			{
				t.Errorf("Unknown gauge: %s", gauge.Name())
			}
		}
	})
}

func TestMetrics_EachCounter(t *testing.T) {
	m := NewMetrics(nil, 10, slog.Default())

	m.RegisterCounter("test_counter", "First")
	m.RegisterCounter("test_counter_2", "Second")

	m.Counter("test_counter").Inc()
	m.Counter("test_counter_2").Add(3)

	m.EachCounter(func(counter *Counter) {
		switch counter.Name() {
		case "test_counter":
			{
				assert.Equal(t, uint64(1), counter.Value())
			}
		case "test_counter_2":
			{
				assert.Equal(t, uint64(3), counter.Value())
			}
		default:
			{
				t.Errorf("Unknown counter: %s", counter.Name())
			}
		}
	})
}
