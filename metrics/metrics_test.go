package metrics

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/anycable/anycable-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

type signalWriter struct {
	ran chan struct{}
}

func (w *signalWriter) Run(interval int) error {
	close(w.ran)
	return nil
}

func (w *signalWriter) Write(m *Metrics) error { return nil }

func (w *signalWriter) Stop() {}

// Regression test: with a dedicated metrics HTTP server configured, Run() must
// still reach the interval writers / rotation loop. Previously the server was
// started synchronously and its blocking Serve starved the rotation loop, so
// metrics logging and StatsD were silently disabled.
func TestRunWithDedicatedServerStartsRotation(t *testing.T) {
	w := &signalWriter{ran: make(chan struct{})}
	m := NewMetrics([]IntervalWriter{w}, 1, slog.Default())

	srv, err := server.NewServer("127.0.0.1", "0", server.SSL, 0)
	require.NoError(t, err)
	m.server = srv

	done := make(chan error, 1)
	go func() { done <- m.Run() }()
	defer m.Shutdown(context.Background()) // nolint:errcheck

	select {
	case <-w.ran:
		// rotation started — the server did not block Run
	case err := <-done:
		t.Fatalf("Run returned before starting the interval writers: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("rotation loop never started: the metrics server start is blocking Run")
	}
}
