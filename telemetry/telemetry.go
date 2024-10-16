package telemetry

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/anycable/anycable-go/config"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/mrb"
	"github.com/anycable/anycable-go/version"
	"github.com/hofstadter-io/cinful"
	"github.com/posthog/posthog-go"
	"golang.org/x/exp/slog"

	nanoid "github.com/matoous/go-nanoid"
)

const (
	usageMeasurementDelayMinutes = 30
)

type Tracker struct {
	id          string
	client      *http.Client
	fingerprint string

	// Remote service configuration
	url       string
	authToken string

	instrumenter *metrics.Metrics
	config       *config.Config
	timer        *time.Timer

	closed bool

	mu     sync.Mutex
	logger *slog.Logger

	// Observed metrics values
	observations   map[string]interface{}
	customizations map[string]string
}

func NewTracker(instrumenter *metrics.Metrics, c *config.Config, tc *Config) *Tracker {
	id, _ := nanoid.Nanoid(8)

	client := &http.Client{}

	logLevel := slog.LevelInfo

	if tc.Debug {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))

	fingerprint := clusterFingerprint(c)

	return &Tracker{
		client:         client,
		url:            tc.Endpoint,
		authToken:      tc.Token,
		logger:         logger,
		config:         c,
		instrumenter:   instrumenter,
		id:             id,
		fingerprint:    fingerprint,
		observations:   make(map[string]interface{}),
		customizations: tc.CustomProps,
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

	t.client.CloseIdleConnections()

	return nil
}

func (t *Tracker) Send(event string, props map[string]interface{}) {
	t.logger.Debug("send telemetry event", "event", event)

	// Avoid storing IP address
	props["$ip"] = nil
	props["distinct_id"] = t.id
	props["cluster-fingerprint"] = t.fingerprint
	props["event"] = event

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	payload, err := json.Marshal(props)

	if err != nil {
		t.logger.Debug("failed to marshal telemetry payload", "err", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.url, bytes.NewReader(payload))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")

	if t.authToken != "" {
		req.Header.Set("Authorization", t.authToken)
	}

	res, err := t.client.Do(req)

	if err != nil {
		if ctx.Err() != nil {
			t.logger.Debug("timed out to send telemetry data")
			return
		}

		t.logger.Debug("failed to perform telemetry request", "err", err)
		return
	}

	defer res.Body.Close()

	if res.StatusCode == http.StatusUnauthorized {
		t.logger.Debug("telemetry authenticated failed")
		return
	}

	if res.StatusCode != http.StatusOK {
		t.logger.Debug("telemetry request failed", "status", res.StatusCode)
		return
	}
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
	return t.appProperties()
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
	props.Set("enats", t.config.EmbeddedNats.Enabled)
	props.Set("broadcast", strings.Join(t.config.BroadcastAdapters, ","))
	props.Set("pubsub", t.config.PubSubAdapter)
	props.Set("broker", t.config.Broker.Adapter)
	props.Set("ssl", t.config.Server.SSL.Available())
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

	// Custom properties
	for k, v := range t.customizations {
		props.Set(k, v)
	}

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

// Try to generate a unique cluster fingerprint to identify events
// from different instances of the same cluster.
func clusterFingerprint(c *config.Config) string {
	platformID := platformServiceID()

	if platformID != "" {
		return generateDigest("P", platformID)
	}

	platform := guessPlatform()
	// Explicitly set env vars
	env := anycableEnvVarsList()
	// Command line arguments
	opts := anycableCLIArgs()
	// File configuration as a string
	file := anycableFileConfig(c.ConfigFilePath)

	// Likely development environment
	if env == "" && opts == "" && file == "" && platform == "" {
		return "default"
	}

	return generateDigest(
		"C",
		platform,
		env,
		opts,
		file,
	)
}

func platformServiceID() string {
	if id, ok := os.LookupEnv("FLY_APP_NAME"); ok {
		return id
	}

	if id, ok := os.LookupEnv("HEROKU_APP_ID"); ok {
		return id
	}

	if id, ok := os.LookupEnv("RENDER_SERVICE_ID"); ok {
		return id
	}

	if id, ok := os.LookupEnv("HATCHBOX_APP_NAME"); ok {
		return id
	}

	if id, ok := os.LookupEnv("K_SERVICE"); ok {
		return id
	}

	return ""
}

func generateDigest(prefix string, parts ...string) string {
	h := sha256.New()

	for _, part := range parts {
		if part != "" {
			h.Write([]byte(part))
		}
	}

	return fmt.Sprintf("%s%x", prefix, h.Sum(nil))
}

// Return a sorted list of AnyCable environment variables.
func anycableEnvVarsList() string {
	pairs := os.Environ()
	vars := []string{}

	for _, pair := range pairs {
		if strings.HasPrefix(pair, "ANYCABLE") {
			vars = append(vars, pair)
		}
	}

	slices.Sort(vars)

	return strings.Join(vars, ",")
}

// Return a sorted list of AnyCable CLI arguments.
func anycableCLIArgs() string {
	args := os.Args[1:]
	slices.Sort(args)

	return strings.Join(args, ",")
}

// Return the contents of AnyCable configuration file if any.
func anycableFileConfig(path string) string {
	if path == "" {
		return ""
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return string(bytes)
}
