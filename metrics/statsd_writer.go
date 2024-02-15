package metrics

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/smira/go-statsd"
)

type StatsdConfig struct {
	Host          string
	Prefix        string
	TagFormat     string
	MaxPacketSize int
}

type StatsdLogger struct {
	log *slog.Logger
}

func (lg *StatsdLogger) Printf(msg string, args ...interface{}) {
	msg = strings.TrimPrefix(msg, "[STATSD] ")
	// Statsd only prints errors and warnings
	if strings.Contains(msg, "Error") {
		lg.log.Error(fmt.Sprintf(msg, args...))
	} else {
		lg.log.Warn(fmt.Sprintf(msg, args...))
	}
}

func NewStatsdConfig() StatsdConfig {
	return StatsdConfig{Prefix: "anycable_go.", MaxPacketSize: 1400, TagFormat: "datadog"}
}

func (c StatsdConfig) Enabled() bool {
	return c.Host != ""
}

type StatsdWriter struct {
	client *statsd.Client
	config StatsdConfig
	tags   map[string]string

	log *slog.Logger
	mu  sync.Mutex
}

var _ IntervalWriter = (*StatsdWriter)(nil)

func NewStatsdWriter(c StatsdConfig, tags map[string]string, l *slog.Logger) *StatsdWriter {
	return &StatsdWriter{config: c, tags: tags, log: l}
}

func (sw *StatsdWriter) Run(interval int) error {
	sl := StatsdLogger{sw.log.With("service", "statsd")}
	opts := []statsd.Option{
		statsd.MaxPacketSize(sw.config.MaxPacketSize),
		statsd.MetricPrefix(sw.config.Prefix),
		statsd.Logger(&sl),
	}

	var tagsInfo string

	if sw.tags != nil {
		tagsStyle, err := resolveTagsStyle(sw.config.TagFormat)
		if err != nil {
			return err
		}

		tags := convertTags(sw.tags)
		opts = append(opts,
			statsd.TagStyle(tagsStyle),
			statsd.DefaultTags(tags...),
		)

		tagsInfo = fmt.Sprintf(", tags=%v, style=%s", sw.tags, sw.config.TagFormat)
	}

	sw.client = statsd.NewClient(
		sw.config.Host,
		opts...,
	)

	sw.log.Info(
		fmt.Sprintf(
			"Send statsd metrics to %s with every %vs (prefix=%s%s)",
			sw.config.Host, interval, sw.config.Prefix, tagsInfo,
		),
	)

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

func resolveTagsStyle(name string) (*statsd.TagFormat, error) {
	switch name {
	case "datadog":
		return statsd.TagFormatDatadog, nil
	case "influxdb":
		return statsd.TagFormatInfluxDB, nil
	case "graphite":
		return statsd.TagFormatGraphite, nil
	}

	return nil, fmt.Errorf("unknown StatsD tags format: %s", name)
}

func convertTags(tags map[string]string) []statsd.Tag {
	buf := make([]statsd.Tag, len(tags))
	i := 0

	for k, v := range tags {
		buf[i] = statsd.StringTag(k, v)
		i++
	}

	return buf
}
