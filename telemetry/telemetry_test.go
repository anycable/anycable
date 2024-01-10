package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/anycable/anycable-go/config"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/version"
	"github.com/posthog/posthog-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockClient struct {
	closed   bool
	captured []posthog.Capture
}

func NewMockClient() *MockClient {
	return &MockClient{captured: []posthog.Capture{}}
}

func (c *MockClient) Enqueue(msg posthog.Message) error {
	c.captured = append(c.captured, msg.(posthog.Capture))

	return nil
}

func (c *MockClient) Close() error {
	c.closed = true

	return nil
}

func (c *MockClient) GetAllFlags(flags posthog.FeatureFlagPayloadNoKey) (map[string]interface{}, error) {
	return nil, nil
}

func (c *MockClient) GetFeatureFlag(flag posthog.FeatureFlagPayload) (interface{}, error) {
	return nil, nil
}

func (c *MockClient) GetFeatureFlags() ([]posthog.FeatureFlag, error) {
	return nil, nil
}

func (c *MockClient) IsFeatureEnabled(flag posthog.FeatureFlagPayload) (interface{}, error) {
	return nil, nil
}

func (c *MockClient) ReloadFeatureFlags() error {
	return nil
}

func TestTracking(t *testing.T) {
	mconfig := metrics.NewConfig()
	metrics, _ := metrics.NewFromConfig(&mconfig)

	metrics.RegisterGauge("clients_num", "")
	metrics.RegisterGauge("mem_sys_bytes", "")
	metrics.GaugeSet("clients_num", 10)
	metrics.GaugeSet("mem_sys_bytes", 100)

	t.Setenv("AWS_EXECUTION_ENV", "AWS_ECS_FARGATE")

	conf := config.NewConfig()
	tracker := NewTracker(metrics, &conf, &Config{})
	defer tracker.Shutdown(context.Background()) // nolint: errcheck

	client := NewMockClient()
	tracker.client = client

	tracker.Collect()

	require.Equal(t, 1, len(client.captured))

	event := client.captured[0]

	assert.Equal(t, "boot", event.Event)
	assert.Equal(t, version.Version(), event.Properties["version"])

	time.Sleep(100 * time.Millisecond)

	metrics.GaugeSet("clients_num", 14)
	metrics.GaugeSet("mem_sys_bytes", 90)

	tracker.observeUsage()
	tracker.collectUsage()

	require.Equal(t, 2, len(client.captured))

	event = client.captured[1]

	assert.Equal(t, "usage", event.Event)
	assert.Equal(t, "ecs-fargate", event.Properties["deploy"])
	assert.Equal(t, 14, int(event.Properties["clients_max"].(uint64)))
	assert.Equal(t, 100, int(event.Properties["mem_sys_max"].(uint64)))

	require.NoError(t, tracker.Shutdown(context.Background()))

	assert.True(t, client.closed)
}
