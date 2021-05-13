/*
Package statsd implements high-performance, zero-allocation statsd client.

Go statsd client library with zero allocation overhead, great performance and automatic
reconnects.

With statsd architecture aggregation is performed on statsd server side (e.g. using
high-performance servers like statsite), so application emits many metrics per user action.
Performance of statsd client library is critical to introduce as little overhead as possible.

Client has zero memory allocation per metric being sent, architecture is the following:

 * there's ring of buffers, each buffer is UDP packet
 * buffer is taken from the pool, filled with metrics, passed on to the network delivery and
   passed back to the pool
 * buffer is flushed either when it is full or when flush period comes (e.g. every 100ms)
 * separate goroutine is handling network operations: sending UDP packets and reconnecting UDP socket
   (to handle statsd DNS address change)
 * when metric is serialized, zero allocation methods are used to avoid `reflect` and memory allocation

Ideas were borrowed from the following stastd clients:

 * https://github.com/quipo/statsd
 * https://github.com/Unix4ever/statsd
 * https://github.com/alexcesaro/statsd/
 * https://github.com/armon/go-metrics

Usage

Initialize client instance with options, one client per application is usually enough:

    client := statsd.NewClient("localhost:8125",
        statsd.MaxPacketSize(1400),
        statsd.MetricPrefix("web."))

Send metrics as events happen in the application, metrics will be packed together and
delivered to statsd server:

    start := time.Now()
    client.Incr("requests.http", 1)
    ...
    client.PrecisionTiming("requests.route.api.latency", time.Since(start))

Shutdown client during application shutdown to flush all the pending metrics:

    client.Close()

Tagging

Metrics could be tagged to support aggregation on TSDB side. go-statsd supports
tags in InfluxDB and Datadog formats. Format and default tags (applied to every
metric) are passed as options to the client initialization:

    client := statsd.NewClient("localhost:8125",
        statsd.TagStyle(TagFormatDatadog),
        statsd.DefaultTags(statsd.StringTag("app", "billing")))

For every metric sent, tags could be added as the last argument(s) to the function
call:

    client.Incr("request", 1,
        statsd.StringTag("protocol", "http"), statsd.IntTag("port", 80))
*/
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
