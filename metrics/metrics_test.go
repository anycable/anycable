package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetricsSnapshot(t *testing.T) {
	m := NewMetrics(nil)

	m.RegisterCounter("test_count")
	m.RegisterGauge("test_gauge")

	for i := 0; i < 1000; i++ {
		m.Counter("test_count").Inc()
	}

	m.Gauge("test_gauge").Set(123)

	m.updateSnapshot()

	assert.Equal(t, int64(1000), m.Snapshot()["test_count"])
	assert.Equal(t, int64(123), m.Snapshot()["test_gauge"])

	m.Counter("test_count").Inc()

	m.updateSnapshot()

	assert.Equal(t, int64(1), m.Snapshot()["test_count"])
	assert.Equal(t, int64(123), m.Snapshot()["test_gauge"])
}
