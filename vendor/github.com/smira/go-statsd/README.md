[![Build Status](https://travis-ci.org/smira/go-statsd.svg?branch=master)](https://travis-ci.org/smira/go-statsd)
[![Documentation](https://godoc.org/github.com/smira/go-statsd?status.svg)](http://godoc.org/github.com/smira/go-statsd)
[![Go Report Card](https://goreportcard.com/badge/github.com/smira/go-statsd)](https://goreportcard.com/report/github.com/smira/go-statsd)
[![codecov](https://codecov.io/gh/smira/go-statsd/branch/master/graph/badge.svg)](https://codecov.io/gh/smira/go-statsd)
[![License](https://img.shields.io/github/license/smira/go-statsd.svg?maxAge=2592000)](https://github.com/smira/go-statsd/LICENSE)
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fsmira%2Fgo-statsd.svg?type=shield)](https://app.fossa.io/projects/git%2Bgithub.com%2Fsmira%2Fgo-statsd?ref=badge_shield)

Go statsd client library with zero allocation overhead, great performance and automatic
reconnects.

Client has zero memory allocation per metric sent:

* ring of buffers, each buffer is UDP packet
* buffer is taken from the pool, filled with metrics, passed on to the network delivery and
returned to the pool
* buffer is flushed either when it is full or when flush period comes (e.g. every 100ms)
* separate goroutines handle network operations: sending UDP packets and reconnecting UDP socket
* when metric is serialized, zero allocation operations are used to avoid `reflect` and temporary buffers

## Zero memory allocation

As metrics could be sent by the application at very high rate (e.g. hundreds of metrics per one request),
it is important that sending metrics doesn't cause any additional GC or CPU pressure. `go-statsd` is using
buffer pools and it tries to avoid allocations while building statsd packets.

## Reconnecting to statsd

With modern container-based platforms with dynamic DNS statsd server might change its address when container
gets rescheduled. As statsd packets are delivered over UDP, there's no easy way for the client to figure out
that packets are going nowhere. `go-statsd` supports configurable reconnect interval which forces DNS resolution.

While client is reconnecting, metrics are still processed and buffered.

## Dropping metrics

When buffer pool is exhausted, `go-statsd` starts dropping packets. Number of dropped packets is reported via
`Client.GetLostPackets()` and every minute logged using `log.Printf()`. Usually packets should never be dropped,
if that happens it's usually signal of enormous metric volume.

## Stastd server

Any statsd-compatible server should work well with `go-statsd`, [statsite](https://github.com/statsite/statsite) works
exceptionally well as it has great performance and low memory footprint even with huge number of metrics.

## Usage

Initialize client instance with options, one client per application is usually enough:

```go
client := statsd.NewClient("localhost:8125",
    statsd.MaxPacketSize(1400),
    statsd.MetricPrefix("web."))
```

Send metrics as events happen in the application, metrics will be packed together and
delivered to statsd server:

```go
start := time.Now()
client.Incr("requests.http", 1)
// ...
client.PrecisionTiming("requests.route.api.latency", time.Since(start))
```

Shutdown client during application shutdown to flush all the pending metrics:

```go
client.Close()
```

## Tagging

Metrics could be tagged to support aggregation on TSDB side. go-statsd supports
tags in [InfluxDB](https://github.com/influxdata/telegraf/tree/master/plugins/inputs/statsd)
, [Datadog](https://docs.datadoghq.com/developers/dogstatsd/#datagram-format)
and [Graphite](https://graphite.readthedocs.io/en/latest/tags.html) formats.
Format and default tags (applied to every metric) are passed as options
to the client initialization:

```go
client := statsd.NewClient("localhost:8125",
    statsd.TagStyle(TagFormatDatadog),
    statsd.DefaultTags(statsd.StringTag("app", "billing")))
```

For every metric sent, tags could be added as the last argument(s) to the function
call:

```go
client.Incr("request", 1,
    statsd.StringTag("procotol", "http"), statsd.IntTag("port", 80))
```


## Benchmark

[Benchmark](https://github.com/smira/go-statsd-benchmark) comparing several clients:

* https://github.com/alexcesaro/statsd/ (`Alexcesaro`)
* this client (`GoStatsd`)
* https://github.com/cactus/go-statsd-client (`Cactus`)
* https://github.com/peterbourgon/g2s (`G2s`)
* https://github.com/quipo/statsd (`Quipo`)
* https://github.com/Unix4ever/statsd (`Unix4ever`)

Benchmark results:

    BenchmarkAlexcesaro-12    	 5000000	       333 ns/op	       0 B/op	       0 allocs/op
    BenchmarkGoStatsd-12      	10000000	       230 ns/op	      23 B/op	       0 allocs/op
    BenchmarkCactus-12        	 3000000	       604 ns/op	       5 B/op	       0 allocs/op
    BenchmarkG2s-12           	  200000	      7499 ns/op	     576 B/op	      21 allocs/op
    BenchmarkQuipo-12         	 1000000	      1048 ns/op	     384 B/op	       7 allocs/op
    BenchmarkUnix4ever-12        1000000	      1695 ns/op	     408 B/op	      18 allocs/op

## Origins

Ideas were borrowed from the following stastd clients:

* https://github.com/quipo/statsd (MIT License, https://github.com/quipo/statsd/blob/master/LICENSE)
* https://github.com/Unix4ever/statsd (MIT License, https://github.com/Unix4ever/statsd/blob/master/LICENSE)
* https://github.com/alexcesaro/statsd/ (MIT License, https://github.com/alexcesaro/statsd/blob/master/LICENSE)
* https://github.com/armon/go-metrics (MIT License, https://github.com/armon/go-metrics/blob/master/LICENSE)

## Talks

I gave a talk about design and optimizations which went into go-statsd at
[Gophercon Russia 2018](https://www.gophercon-russia.ru/):
[slides](https://talks.godoc.org/github.com/smira/gopherconru2018/go-statsd.slide),
[source](https://github.com/smira/gopherconru2018).

## License

License is [MIT License](LICENSE).


[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fsmira%2Fgo-statsd.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fsmira%2Fgo-statsd?ref=badge_large)
