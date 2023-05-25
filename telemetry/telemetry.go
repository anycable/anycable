package telemetry

import (
	"runtime"
	"sync"
	"time"

	"github.com/anycable/anycable-go/config"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mrb"
	"github.com/anycable/anycable-go/version"
	"github.com/apex/log"
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

func NewTracker(instrumenter *metrics.Metrics, c *config.Config, tc *Config) *Tracker {
	client, _ := posthog.NewWithConfig(tc.Token, posthog.Config{Endpoint: tc.Endpoint})

	id, _ := nanoid.Nanoid(8)

	return &Tracker{
		client:       client,
		config:       c,
		instrumenter: instrumenter,
		id:           id,
		observations: make(map[string]interface{}),
	}
}

func (t *Tracker) Announce() {
	log.WithField("context", "main").Info("Anonymized telemetry is on. Learn more: https://docs.anycable.io/anycable-go/telemetry")
}

func (t *Tracker) Collect() {
	t.Send("boot", t.bootProperties())

	go t.monitorUsage()

	t.timer = time.AfterFunc(usageMeasurementDelayMinutes*time.Minute, t.collectUsage)
}

func (t *Tracker) Shutdown() error {
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

	t.Send("usage", t.observations)
}

func (t *Tracker) bootProperties() map[string]interface{} {
	props := posthog.NewProperties()

	// Basic info
	props.Set("version", version.Version())
	props.Set("os", runtime.GOOS)
	props.Set("mruby", mrb.Supported())

	// Features
	props.Set("jwt", t.config.JWT.Enabled())
	props.Set("turbo", t.config.Rails.TurboRailsKey != "")
	props.Set("turbo-ct", t.config.Rails.TurboRailsClearText)
	props.Set("cr", t.config.Rails.CableReadyKey != "")
	props.Set("cr-ct", t.config.Rails.CableReadyClearText)
	props.Set("enats", t.config.EmbedNats)
	props.Set("broadcast", t.config.BroadcastAdapter)
	props.Set("pubsub", t.config.PubSubAdapter)
	props.Set("broker", t.config.BrokerAdapter)
	props.Set("ssl", t.config.SSL.Available())
	props.Set("mrb-printer", t.config.Metrics.LogFormatterEnabled())
	props.Set("statsd", t.config.Metrics.Statsd.Enabled())
	props.Set("prom", t.config.Metrics.HTTPEnabled())

	return props
}
