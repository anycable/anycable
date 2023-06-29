package telemetry

import (
	"context"
	"maps"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/anycable/anycable-go/config"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mrb"
	"github.com/anycable/anycable-go/version"
	"github.com/hofstadter-io/cinful"
	"github.com/posthog/posthog-go"

	nanoid "github.com/matoous/go-nanoid"
)

const (
	usageMeasurementDelayMinutes = 30
)

type Tracker struct {
	id           string
	client       posthog.Client
	instrumenter *metrics.Metrics
	config       *config.Config
	timer        *time.Timer

	closed bool

	mu sync.Mutex

	// Observed metrics values
	observations map[string]interface{}
}

type noopLogger struct{}

func (l noopLogger) Logf(format string, args ...interface{})   {}
func (l noopLogger) Errorf(format string, args ...interface{}) {}

func NewTracker(instrumenter *metrics.Metrics, c *config.Config, tc *Config) *Tracker {
	client, _ := posthog.NewWithConfig(tc.Token, posthog.Config{
		Endpoint: tc.Endpoint,
		// set to no-op to avoid logging
		Logger: noopLogger{},
	})

	id, _ := nanoid.Nanoid(8)

	return &Tracker{
		client:       client,
		config:       c,
		instrumenter: instrumenter,
		id:           id,
		observations: make(map[string]interface{}),
	}
}

func (t *Tracker) Announce() string {
	return "Anonymized telemetry is on. Learn more: https://docs.anycable.io/anycable-go/telemetry"
}

func (t *Tracker) Collect() {
	t.Send("boot", t.bootProperties())

	go t.monitorUsage()

	t.mu.Lock()
	defer t.mu.Unlock()

	t.timer = time.AfterFunc(usageMeasurementDelayMinutes*time.Minute, t.collectUsage)
}

func (t *Tracker) Shutdown(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true

	if t.timer != nil {
		t.timer.Stop()
	}

	return t.client.Close()
}

func (t *Tracker) Send(event string, props map[string]interface{}) {
	// Avoid storing IP address
	props["$ip"] = nil
	props["distinct_id"] = t.id

	_ = t.client.Enqueue(posthog.Capture{
		DistinctId: t.id,
		Event:      event,
		Properties: props,
	})
}

func (t *Tracker) monitorUsage() {
	for {
		t.mu.Lock()
		if t.closed {
			t.mu.Unlock()
			return
		}
		t.mu.Unlock()

		t.observeUsage()

		time.Sleep(1 * time.Minute)
	}
}

func (t *Tracker) observeUsage() {
	t.storeObservation("clients_max", t.instrumenter.Gauge("clients_num").Value())
	t.storeObservation("mem_sys_max", t.instrumenter.Gauge("mem_sys_bytes").Value())
}

func (t *Tracker) storeObservation(key string, val uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if oldVal, ok := t.observations[key]; ok {
		if val > oldVal.(uint64) {
			t.observations[key] = val
		}
	} else {
		t.observations[key] = val
	}
}

func (t *Tracker) collectUsage() {
	t.mu.Lock()
	defer t.mu.Unlock()

	props := t.appProperties()
	maps.Copy(props, t.observations)

	t.Send("usage", props)

	// Reset observations
	t.observations = make(map[string]interface{})
	t.timer = time.AfterFunc(usageMeasurementDelayMinutes*time.Minute, t.collectUsage)
}

func (t *Tracker) bootProperties() map[string]interface{} {
	props := posthog.NewProperties()

	props.Set("version", version.Version())
	props.Set("os", runtime.GOOS)
	props.Set("pro", true)

	return props
}

func (t *Tracker) appProperties() map[string]interface{} {
	props := posthog.NewProperties()

	// Basic info
	props.Set("version", version.Version())
	props.Set("os", runtime.GOOS)
	props.Set("mruby", mrb.Supported())

	ciVendor := cinful.Info()
	props.Set("ci", ciVendor != nil)

	if ciVendor != nil {
		props.Set("ci-name", ciVendor.Name)
	}

	props.Set("deploy", guessPlatform())

	// Features
	props.Set("has-secret", t.config.Secret != "")
	props.Set("no-auth", t.config.SkipAuth)
	props.Set("jwt", t.config.JWT.Enabled())
	props.Set("public-streams", t.config.Streams.Public)
	props.Set("turbo", t.config.Streams.Turbo)
	props.Set("cr", t.config.Streams.CableReady)
	props.Set("enats", t.config.EmbedNats)
	props.Set("broadcast", t.config.BroadcastAdapter)
	props.Set("pubsub", t.config.PubSubAdapter)
	props.Set("broker", t.config.BrokerAdapter)
	props.Set("ssl", t.config.SSL.Available())
	props.Set("mrb-printer", t.config.Metrics.LogFormatterEnabled())
	props.Set("statsd", t.config.Metrics.Statsd.Enabled())
	props.Set("prom", t.config.Metrics.HTTPEnabled())
	props.Set("rpc-impl", t.config.RPC.Impl())

	// AnyCable+
	name, ok := os.LookupEnv("ANYCABLEPLUS_APP_NAME")
	props.Set("plus", ok)
	if ok {
		props.Set("plus-name", name)
	}

	// PRO features
	props.Set("pro", true)
	props.Set("apollo", t.config.GraphQL.Enabled())
	props.Set("ocpp", t.config.OCPP.Enabled())
	props.Set("auto-rpc", t.config.RPC.Concurrency == 0)
	props.Set("lp", t.config.LongPolling.Enabled)

	return props
}

func guessPlatform() string {
	if _, ok := os.LookupEnv("FLY_APP_NAME"); ok {
		return "fly"
	}

	if _, ok := os.LookupEnv("HEROKU_APP_ID"); ok {
		return "heroku"
	}

	if _, ok := os.LookupEnv("RENDER_SERVICE_ID"); ok {
		return "render"
	}

	if _, ok := os.LookupEnv("HATCHBOX_APP_NAME"); ok {
		return "hatchbox"
	}

	if awsEnv, ok := os.LookupEnv("AWS_EXECUTION_ENV"); ok {
		if awsEnv == "AWS_ECS_FARGATE" {
			return "ecs-fargate"
		}

		if awsEnv == "AWS_ECS_EC2" {
			return "ecs-ec2"
		}

		return "ecs"
	}

	if _, ok := os.LookupEnv("ECS_CONTAINER_METADATA_URI"); ok {
		return "ecs"
	}

	if _, ok := os.LookupEnv("ECS_CONTAINER_METADATA_URI_V4"); ok {
		return "ecs"
	}

	if _, ok := os.LookupEnv("K_SERVICE"); ok {
		return "cloud-run"
	}

	return ""
}
