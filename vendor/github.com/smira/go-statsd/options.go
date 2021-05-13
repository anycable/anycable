package statsd

/*

Copyright (c) 2017 Andrey Smirnov

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

*/

import (
	"time"
)

// Default settings
const (
	DefaultMaxPacketSize     = 1432
	DefaultMetricPrefix      = ""
	DefaultFlushInterval     = 100 * time.Millisecond
	DefaultReconnectInterval = time.Duration(0)
	DefaultReportInterval    = time.Minute
	DefaultRetryTimeout      = 5 * time.Second
	DefaultLogPrefix         = "[STATSD] "
	DefaultBufPoolCapacity   = 20
	DefaultSendQueueCapacity = 10
	DefaultSendLoopCount     = 1
)

// SomeLogger defines logging interface that allows using 3rd party loggers
// (e.g. github.com/sirupsen/logrus) with this Statsd client.
type SomeLogger interface {
	Printf(fmt string, args ...interface{})
}

// ClientOptions are statsd client settings
type ClientOptions struct {
	// Addr is statsd server address in "host:port" format
	Addr string

	// MetricPrefix is metricPrefix to prepend to every metric being sent
	//
	// If not set defaults to empty string
	MetricPrefix string

	// MaxPacketSize is maximum UDP packet size
	//
	// Safe value is 1432 bytes, if your network supports jumbo frames,
	// this value could be raised up to 8960 bytes
	MaxPacketSize int

	// FlushInterval controls flushing incomplete UDP packets which makes
	// sure metric is not delayed longer than FlushInterval
	//
	// Default value is 100ms, setting FlushInterval to zero disables flushing
	FlushInterval time.Duration

	// ReconnectInterval controls UDP socket reconnects
	//
	// Reconnecting is important to follow DNS changes, e.g. in
	// dynamic container environments like K8s where statsd server
	// instance might be relocated leading to new IP address.
	//
	// By default reconnects are disabled
	ReconnectInterval time.Duration

	// RetryTimeout controls how often client should attempt reconnecting
	// to statsd server on failure
	//
	// Default value is 5 seconds
	RetryTimeout time.Duration

	// ReportInterval instructs client to report number of packets lost
	// each interval via Logger
	//
	// By default lost packets are reported every minute, setting to zero
	// disables reporting
	ReportInterval time.Duration

	// Logger is used by statsd client to report errors and lost packets
	//
	// If not set, default logger to stderr with metricPrefix `[STATSD] ` is being used
	Logger SomeLogger

	// BufPoolCapacity controls size of pre-allocated buffer cache
	//
	// Each buffer is MaxPacketSize. Cache allows to avoid allocating
	// new buffers during high load
	//
	// Default value is DefaultBufPoolCapacity
	BufPoolCapacity int

	// SendQueueCapacity controls length of the queue of packet ready to be sent
	//
	// Packets might stay in the queue during short load bursts or while
	// client is reconnecting to statsd
	//
	// Default value is DefaultSendQueueCapacity
	SendQueueCapacity int

	// SendLoopCount controls number of goroutines sending UDP packets
	//
	// Default value is 1, so packets are sent from single goroutine, this
	// value might need to be bumped under high load
	SendLoopCount int

	// TagFormat controls formatting of StatsD tags
	//
	// If tags are not used, value of this setting isn't used.
	//
	// There are two predefined formats: for InfluxDB and Datadog, default
	// format is InfluxDB tag format.
	TagFormat *TagFormat

	// DefaultTags is a list of tags to be applied to every metric
	DefaultTags []Tag
}

// Option is type for option transport
type Option func(c *ClientOptions)

// MetricPrefix is metricPrefix to prepend to every metric being sent
//
// Usually metrics are prefixed with app name, e.g. `app.`.
// To avoid providing this metricPrefix for every metric being collected,
// and to enable shared libraries to collect metric under app name,
// use MetricPrefix to set global metricPrefix for all the app metrics,
// e.g. `MetricPrefix("app.")`.
//
// If not set defaults to empty string
func MetricPrefix(prefix string) Option {
	return func(c *ClientOptions) {
		c.MetricPrefix = prefix
	}
}

// MaxPacketSize control maximum UDP packet size
//
// Default value is DefaultMaxPacketSize
func MaxPacketSize(packetSize int) Option {
	return func(c *ClientOptions) {
		c.MaxPacketSize = packetSize
	}
}

// FlushInterval controls flushing incomplete UDP packets which makes
// sure metric is not delayed longer than FlushInterval
//
// Default value is 100ms, setting FlushInterval to zero disables flushing
func FlushInterval(interval time.Duration) Option {
	return func(c *ClientOptions) {
		c.FlushInterval = interval
	}
}

// ReconnectInterval controls UDP socket reconnects
//
// Reconnecting is important to follow DNS changes, e.g. in
// dynamic container environments like K8s where statsd server
// instance might be relocated leading to new IP address.
//
// By default reconnects are disabled
func ReconnectInterval(interval time.Duration) Option {
	return func(c *ClientOptions) {
		c.ReconnectInterval = interval
	}
}

// RetryTimeout controls how often client should attempt reconnecting
// to statsd server on failure
//
// Default value is 5 seconds
func RetryTimeout(timeout time.Duration) Option {
	return func(c *ClientOptions) {
		c.RetryTimeout = timeout
	}
}

// ReportInterval instructs client to report number of packets lost
// each interval via Logger
//
// By default lost packets are reported every minute, setting to zero
// disables reporting
func ReportInterval(interval time.Duration) Option {
	return func(c *ClientOptions) {
		c.ReportInterval = interval
	}
}

// Logger is used by statsd client to report errors and lost packets
//
// If not set, default logger to stderr with metricPrefix `[STATSD] ` is being used
func Logger(logger SomeLogger) Option {
	return func(c *ClientOptions) {
		c.Logger = logger
	}
}

// BufPoolCapacity controls size of pre-allocated buffer cache
//
// Each buffer is MaxPacketSize. Cache allows to avoid allocating
// new buffers during high load
//
// Default value is DefaultBufPoolCapacity
func BufPoolCapacity(capacity int) Option {
	return func(c *ClientOptions) {
		c.BufPoolCapacity = capacity
	}
}

// SendQueueCapacity controls length of the queue of packet ready to be sent
//
// Packets might stay in the queue during short load bursts or while
// client is reconnecting to statsd
//
// Default value is DefaultSendQueueCapacity
func SendQueueCapacity(capacity int) Option {
	return func(c *ClientOptions) {
		c.SendQueueCapacity = capacity
	}
}

// SendLoopCount controls number of goroutines sending UDP packets
//
// Default value is 1, so packets are sent from single goroutine, this
// value might need to be bumped under high load
func SendLoopCount(threads int) Option {
	return func(c *ClientOptions) {
		c.SendLoopCount = threads
	}
}

// TagStyle controls formatting of StatsD tags
//
// There are two predefined formats: for InfluxDB and Datadog, default
// format is InfluxDB tag format.
func TagStyle(style *TagFormat) Option {
	return func(c *ClientOptions) {
		c.TagFormat = style
	}
}

// DefaultTags defines a list of tags to be applied to every metric
func DefaultTags(tags ...Tag) Option {
	return func(c *ClientOptions) {
		c.DefaultTags = tags
	}
}
