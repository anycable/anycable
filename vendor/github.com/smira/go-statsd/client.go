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
	"log"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Client implements statsd client
type Client struct {
	trans        *transport
	metricPrefix string
	defaultTags  []Tag
}

type transport struct {
	maxPacketSize int
	tagFormat     *TagFormat

	bufPool   chan []byte
	buf       []byte
	bufSize   int
	bufLock   sync.Mutex
	sendQueue chan []byte

	shutdown     chan struct{}
	shutdownOnce sync.Once
	shutdownWg   sync.WaitGroup

	lostPacketsPeriod  int64
	lostPacketsOverall int64
}

// NewClient creates new statsd client and starts background processing
//
// Client connects to statsd server at addr ("host:port")
//
// Client settings could be controlled via functions of type Option
func NewClient(addr string, options ...Option) *Client {
	opts := ClientOptions{
		Addr:              addr,
		MetricPrefix:      DefaultMetricPrefix,
		MaxPacketSize:     DefaultMaxPacketSize,
		FlushInterval:     DefaultFlushInterval,
		ReconnectInterval: DefaultReconnectInterval,
		ReportInterval:    DefaultReportInterval,
		RetryTimeout:      DefaultRetryTimeout,
		Logger:            log.New(os.Stderr, DefaultLogPrefix, log.LstdFlags),
		BufPoolCapacity:   DefaultBufPoolCapacity,
		SendQueueCapacity: DefaultSendQueueCapacity,
		SendLoopCount:     DefaultSendLoopCount,
		TagFormat:         TagFormatInfluxDB,
	}

	c := &Client{
		trans: &transport{
			shutdown: make(chan struct{}),
		},
	}
	// 1024 is room for overflow metric
	c.trans.bufSize = opts.MaxPacketSize + 1024

	for _, option := range options {
		option(&opts)
	}

	c.metricPrefix = opts.MetricPrefix
	c.defaultTags = opts.DefaultTags

	c.trans.tagFormat = opts.TagFormat
	c.trans.maxPacketSize = opts.MaxPacketSize
	c.trans.buf = make([]byte, 0, c.trans.bufSize)
	c.trans.bufPool = make(chan []byte, opts.BufPoolCapacity)
	c.trans.sendQueue = make(chan []byte, opts.SendQueueCapacity)

	go c.trans.flushLoop(opts.FlushInterval)

	for i := 0; i < opts.SendLoopCount; i++ {
		c.trans.shutdownWg.Add(1)
		go c.trans.sendLoop(opts.Addr, opts.ReconnectInterval, opts.RetryTimeout, opts.Logger)
	}

	if opts.ReportInterval > 0 {
		c.trans.shutdownWg.Add(1)
		go c.trans.reportLoop(opts.ReportInterval, opts.Logger)
	}

	return c
}

// Close stops the client and all its clones. Calling it on a clone has the
// same effect as calling it on the original client - it is stopped with all
// its clones.
func (c *Client) Close() error {
	c.trans.close()
	return nil
}

func (t *transport) close() {
	t.shutdownOnce.Do(func() {
		close(t.shutdown)
	})
	t.shutdownWg.Wait()
}

// CloneWithPrefix returns a clone of the original client with different metricPrefix.
func (c *Client) CloneWithPrefix(prefix string) *Client {
	clone := *c
	clone.metricPrefix = prefix
	return &clone
}

// CloneWithPrefixExtension returns a clone of the original client with the
// original prefixed extended with the specified string.
func (c *Client) CloneWithPrefixExtension(extension string) *Client {
	clone := *c
	clone.metricPrefix = clone.metricPrefix + extension
	return &clone
}

// GetLostPackets returns number of packets lost during client lifecycle
func (c *Client) GetLostPackets() int64 {
	return atomic.LoadInt64(&c.trans.lostPacketsOverall)
}

// Incr increments a counter metric
//
// Often used to note a particular event, for example incoming web request.
func (c *Client) Incr(stat string, count int64, tags ...Tag) {
	if count != 0 {
		c.trans.bufLock.Lock()
		lastLen := len(c.trans.buf)

		c.trans.buf = append(c.trans.buf, []byte(c.metricPrefix)...)
		c.trans.buf = append(c.trans.buf, []byte(stat)...)
		if c.trans.tagFormat.Placement == TagPlacementName {
			c.trans.buf = c.formatTags(c.trans.buf, tags)
		}
		c.trans.buf = append(c.trans.buf, ':')
		c.trans.buf = strconv.AppendInt(c.trans.buf, count, 10)
		c.trans.buf = append(c.trans.buf, []byte("|c")...)
		if c.trans.tagFormat.Placement == TagPlacementSuffix {
			c.trans.buf = c.formatTags(c.trans.buf, tags)
		}
		c.trans.buf = append(c.trans.buf, '\n')

		c.trans.checkBuf(lastLen)
		c.trans.bufLock.Unlock()
	}
}

// Decr decrements a counter metri
//
// Often used to note a particular event
func (c *Client) Decr(stat string, count int64, tags ...Tag) {
	c.Incr(stat, -count, tags...)
}

// FIncr increments a float counter metric
func (c *Client) FIncr(stat string, count float64, tags ...Tag) {
	if count != 0 {
		c.trans.bufLock.Lock()
		lastLen := len(c.trans.buf)

		c.trans.buf = append(c.trans.buf, []byte(c.metricPrefix)...)
		c.trans.buf = append(c.trans.buf, []byte(stat)...)
		if c.trans.tagFormat.Placement == TagPlacementName {
			c.trans.buf = c.formatTags(c.trans.buf, tags)
		}
		c.trans.buf = append(c.trans.buf, ':')
		c.trans.buf = strconv.AppendFloat(c.trans.buf, count, 'f', -1, 64)
		c.trans.buf = append(c.trans.buf, []byte("|c")...)
		if c.trans.tagFormat.Placement == TagPlacementSuffix {
			c.trans.buf = c.formatTags(c.trans.buf, tags)
		}
		c.trans.buf = append(c.trans.buf, '\n')

		c.trans.checkBuf(lastLen)
		c.trans.bufLock.Unlock()
	}
}

// FDecr decrements a float counter metric
func (c *Client) FDecr(stat string, count float64, tags ...Tag) {
	c.FIncr(stat, -count, tags...)
}

// Timing tracks a duration event, the time delta must be given in milliseconds
func (c *Client) Timing(stat string, delta int64, tags ...Tag) {
	c.trans.bufLock.Lock()
	lastLen := len(c.trans.buf)

	c.trans.buf = append(c.trans.buf, []byte(c.metricPrefix)...)
	c.trans.buf = append(c.trans.buf, []byte(stat)...)
	if c.trans.tagFormat.Placement == TagPlacementName {
		c.trans.buf = c.formatTags(c.trans.buf, tags)
	}
	c.trans.buf = append(c.trans.buf, ':')
	c.trans.buf = strconv.AppendInt(c.trans.buf, delta, 10)
	c.trans.buf = append(c.trans.buf, []byte("|ms")...)
	if c.trans.tagFormat.Placement == TagPlacementSuffix {
		c.trans.buf = c.formatTags(c.trans.buf, tags)
	}
	c.trans.buf = append(c.trans.buf, '\n')

	c.trans.checkBuf(lastLen)
	c.trans.bufLock.Unlock()
}

// PrecisionTiming track a duration event, the time delta has to be a duration
//
// Usually request processing time, time to run database query, etc. are used with
// this metric type.
func (c *Client) PrecisionTiming(stat string, delta time.Duration, tags ...Tag) {
	c.trans.bufLock.Lock()
	lastLen := len(c.trans.buf)

	c.trans.buf = append(c.trans.buf, []byte(c.metricPrefix)...)
	c.trans.buf = append(c.trans.buf, []byte(stat)...)
	if c.trans.tagFormat.Placement == TagPlacementName {
		c.trans.buf = c.formatTags(c.trans.buf, tags)
	}
	c.trans.buf = append(c.trans.buf, ':')
	c.trans.buf = strconv.AppendFloat(c.trans.buf, float64(delta)/float64(time.Millisecond), 'f', -1, 64)
	c.trans.buf = append(c.trans.buf, []byte("|ms")...)
	if c.trans.tagFormat.Placement == TagPlacementSuffix {
		c.trans.buf = c.formatTags(c.trans.buf, tags)
	}
	c.trans.buf = append(c.trans.buf, '\n')

	c.trans.checkBuf(lastLen)
	c.trans.bufLock.Unlock()
}

func (c *Client) igauge(stat string, sign []byte, value int64, tags ...Tag) {
	c.trans.bufLock.Lock()
	lastLen := len(c.trans.buf)

	c.trans.buf = append(c.trans.buf, []byte(c.metricPrefix)...)
	c.trans.buf = append(c.trans.buf, []byte(stat)...)
	if c.trans.tagFormat.Placement == TagPlacementName {
		c.trans.buf = c.formatTags(c.trans.buf, tags)
	}
	c.trans.buf = append(c.trans.buf, ':')
	c.trans.buf = append(c.trans.buf, sign...)
	c.trans.buf = strconv.AppendInt(c.trans.buf, value, 10)
	c.trans.buf = append(c.trans.buf, []byte("|g")...)
	if c.trans.tagFormat.Placement == TagPlacementSuffix {
		c.trans.buf = c.formatTags(c.trans.buf, tags)
	}
	c.trans.buf = append(c.trans.buf, '\n')

	c.trans.checkBuf(lastLen)
	c.trans.bufLock.Unlock()
}

// Gauge sets or updates constant value for the interval
//
// Gauges are a constant data type. They are not subject to averaging,
// and they don’t change unless you change them. That is, once you set a gauge value,
// it will be a flat line on the graph until you change it again. If you specify
// delta to be true, that specifies that the gauge should be updated, not set. Due to the
// underlying protocol, you can't explicitly set a gauge to a negative number without
// first setting it to zero.
func (c *Client) Gauge(stat string, value int64, tags ...Tag) {
	if value < 0 {
		c.igauge(stat, nil, 0, tags...)
	}

	c.igauge(stat, nil, value, tags...)
}

// GaugeDelta sends a change for a gauge
func (c *Client) GaugeDelta(stat string, value int64, tags ...Tag) {
	// Gauge Deltas are always sent with a leading '+' or '-'. The '-' takes care of itself but the '+' must added by hand
	if value < 0 {
		c.igauge(stat, nil, value, tags...)
	} else {
		c.igauge(stat, []byte{'+'}, value, tags...)
	}
}

func (c *Client) fgauge(stat string, sign []byte, value float64, tags ...Tag) {
	c.trans.bufLock.Lock()
	lastLen := len(c.trans.buf)

	c.trans.buf = append(c.trans.buf, []byte(c.metricPrefix)...)
	c.trans.buf = append(c.trans.buf, []byte(stat)...)
	if c.trans.tagFormat.Placement == TagPlacementName {
		c.trans.buf = c.formatTags(c.trans.buf, tags)
	}
	c.trans.buf = append(c.trans.buf, ':')
	c.trans.buf = append(c.trans.buf, sign...)
	c.trans.buf = strconv.AppendFloat(c.trans.buf, value, 'f', -1, 64)
	c.trans.buf = append(c.trans.buf, []byte("|g")...)
	if c.trans.tagFormat.Placement == TagPlacementSuffix {
		c.trans.buf = c.formatTags(c.trans.buf, tags)
	}
	c.trans.buf = append(c.trans.buf, '\n')

	c.trans.checkBuf(lastLen)
	c.trans.bufLock.Unlock()
}

// FGauge sends a floating point value for a gauge
func (c *Client) FGauge(stat string, value float64, tags ...Tag) {
	if value < 0 {
		c.igauge(stat, nil, 0, tags...)
	}

	c.fgauge(stat, nil, value, tags...)
}

// FGaugeDelta sends a floating point change for a gauge
func (c *Client) FGaugeDelta(stat string, value float64, tags ...Tag) {
	if value < 0 {
		c.fgauge(stat, nil, value, tags...)
	} else {
		c.fgauge(stat, []byte{'+'}, value, tags...)
	}
}

// SetAdd adds unique element to a set
//
// Statsd server will provide cardinality of the set over aggregation period.
func (c *Client) SetAdd(stat string, value string, tags ...Tag) {
	c.trans.bufLock.Lock()
	lastLen := len(c.trans.buf)

	c.trans.buf = append(c.trans.buf, []byte(c.metricPrefix)...)
	c.trans.buf = append(c.trans.buf, []byte(stat)...)
	if c.trans.tagFormat.Placement == TagPlacementName {
		c.trans.buf = c.formatTags(c.trans.buf, tags)
	}
	c.trans.buf = append(c.trans.buf, ':')
	c.trans.buf = append(c.trans.buf, []byte(value)...)
	c.trans.buf = append(c.trans.buf, []byte("|s")...)
	if c.trans.tagFormat.Placement == TagPlacementSuffix {
		c.trans.buf = c.formatTags(c.trans.buf, tags)
	}
	c.trans.buf = append(c.trans.buf, '\n')

	c.trans.checkBuf(lastLen)
	c.trans.bufLock.Unlock()
}
