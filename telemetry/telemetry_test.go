package telemetry

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anycable/anycable-go/config"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockClient struct {
	captured []*http.Request
}

func NewMockClient() *MockClient {
	return &MockClient{captured: []*http.Request{}}
}

func (c *MockClient) Do(req *http.Request) (*http.Response, error) {
	c.captured = append(c.captured, req)

	return nil, nil
}

func TestTracking(t *testing.T) {
	mconfig := metrics.NewConfig()
	metrics, _ := metrics.NewFromConfig(&mconfig, slog.Default())

	metrics.RegisterGauge("clients_num", "")
	metrics.RegisterGauge("mem_sys_bytes", "")
	metrics.GaugeSet("clients_num", 10)
	metrics.GaugeSet("mem_sys_bytes", 100)

	t.Setenv("AWS_EXECUTION_ENV", "AWS_ECS_FARGATE")

	captured := []map[string]interface{}{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var event map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &event))
		captured = append(captured, event)
		w.WriteHeader(http.StatusOK)
	}))

	defer ts.Close()

	conf := config.NewConfig()
	tracker := NewTracker(metrics, &conf, &Config{Endpoint: ts.URL})
	defer tracker.Shutdown(context.Background()) // nolint: errcheck

	tracker.Collect()

	require.Equal(t, 1, len(captured))
	event := captured[0]

	assert.Equal(t, "boot", event["event"])
	assert.Equal(t, version.Version(), event["version"])

	time.Sleep(100 * time.Millisecond)

	metrics.GaugeSet("clients_num", 14)
	metrics.GaugeSet("mem_sys_bytes", 90)

	tracker.observeUsage()
	tracker.collectUsage()

	require.Equal(t, 2, len(captured))
	event = captured[1]

	assert.Equal(t, "usage", event["event"])
	assert.Equal(t, "ecs-fargate", event["deploy"])
	assert.Equal(t, 14, int(event["clients_max"].(float64)))
	assert.Equal(t, 100, int(event["mem_sys_max"].(float64)))

	require.NoError(t, tracker.Shutdown(context.Background()))
}
