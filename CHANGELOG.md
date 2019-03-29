# Change log

## master

- Use request id for session uids if available. ([@sponomarev][])

If a load balancer standing in front of WebSocket server assigns `X-Request-ID` header,
this request ID will be used for session identification.

## 0.6.2 (2019-03-25)

- Configure maximum message size via `--max_message_size`. Defaults to 65536 (64kb).

## 0.6.1 (2018-12-21) "X-mas time is here again!" 🎅

- Add HTTP health check endpoint. ([@sponomarev][])

Go to `/health` (you can configure the path via `--health-path`) to see the _health_ message.
You can use this endpoint as readiness/liveness check (e.g. for load balancers).

## 0.6.0 (2018-11-12)

- Add force termination support (by sending the second signal). ([@palkan][])

- Add session ID to outgoing RPC call's metadata. ([@palkan][])

All RPC calls contains the related session ID to metadata (`sid` key).

- Send transmissions to client even if authentication failed. ([@palkan][])

- Fix websocket close status to reflect the reason for the closure. ([@sponomarev][])

- Reduce binary size. ([@sponomarev][])

## 0.6.0-preview6

- Add experimental mruby integration for custom metrics logging. ([@palkan][])

**NOTE:** This feature _experimental_, i.e. the final API may (and likely will) change.

You can write custom Ruby script to implement statistics logging.

For example, to provide Librato-comatible output you can write a custom formatter like this:

```ruby
# my-metrics-formatter.rb
# This MetricsFormatter name is required!
module MetricsFormatter
  KEYS = %w(clients_num clients_unique_num goroutines_num)

  # `data` is a Hash containing all the metrics data
  def self.call(data)
    parts = []

    data.each do |key, value|
      parts << "sample##{key}=#{value}" if KEYS.include?(key)
    end

    parts.join(' ')
  end
end
```

And then use it like this:

```sh
anycable-go --metrics_log_formatter="my-metrics-formatter.rb"

>INFO 2018-04-27T14:11:59.701Z sample#clients_num=0 sample#clients_uniq_num=0 sample#goroutines_num=0
```

## 0.6.0-preview5

- [Fixes [#34](https://github.com/anycable/anycable-go/issues/34)] Fix `panic` when trying to send to a closed channel. ([@palkan][])

- Add `--metrics_host` option. ([@shedimon][])

## 0.6.0-preview4

- Fixed memory leak (caused by leaking mutex goroutines).

- Other minor fixes.

##  0.6.0-preview2

- Add `--metrics_log_interval` option. ([@palkan][])

- Add HTTP `/metrics` endpoint. ([@palkan][])

Serves Prometheus-formatted metrics.

Enabled with `--metrics_http=/metrics` option.

Specify custom port with `--metrics_http_port=1234`.

- Add RPC metrics. ([@palkan][])

- Refactor metrics: add descriptions, change names.  ([@palkan][])

## 0.6.0-preview1

- Add basic metrics.  ([@palkan][])

Added `metrics` package with some basic metrics (_counters_ and _gauges_).

Metrics collected (_cached_) every 15s (will be configurable in the future).

Currently, only _logging_ mode is available (disabled by default): run with `--metrics_log` command to flush stats every 15s to log (with `info` level).

NOTE: _Counter_ values contain _delta_ counts (for the last period).

Example metrics log entry:

```
INFO 2018-03-05T16:20:07.367Z auth_failures_count=0 broadcast_msg_count=228 client_msg_count=2228 clients_num=10000 context=metrics disconnect_queue_size=0 goroutines_num=21273 streams_num=1 uniq_clients_num=10000 unknown_broadcast_msg_count=0 unknown_client_msg_count=0
```

- Add signal handling and graceful shutdown.  ([@palkan][])

When receiving `SIGINT` or `SIGTERM` we:
- Stop the server (to not accept new connections)
- Close all registered sessions (authenticated clients)
- Wait for pending Disconnect requests to complete
- Wait for active RPC calls to finish.

- **[Breaking Change]** New configuration and CLI options.  ([@palkan][])

Environment variables now should be prefixed with `ANYCABLE_`.

`REDIS_URL` and `PORT` env variables are recognized by default.

Setting server address is split into two params: `--port` and `--host` (instead of `--addr`).

Run `anycable-go -h` to learn more.

- New logging format.  ([@palkan][])

Now we use _structured_ logging with the help if [apex/log](https://github.com/apex/log). For example:

```
INFO 2018-03-05T08:44:57.684Z context=main Starting AnyCable unknown
INFO 2018-03-05T08:44:57.684Z context=main Handle WebSocket connections at /cable
INFO 2018-03-05T08:44:57.684Z context=http Starting HTTP server at 0.0.0.0:8080
INFO 2018-03-05T08:44:57.685Z context=rpc RPC pool initialized: 0.0.0.0:50051
INFO 2018-03-05T08:44:57.695Z context=pubsub Subscribed to Redis channel: __anycable__
```

Also, `json` format is supported out-of-the-box (`--log_format=json` or `ANYCABLE_LOG_FORMAT=json`).

- Add unique identifiers to connections.  ([@palkan][])

Helps to identify connections in logs. Will be included into RPC calls in the future releases.

- [Closes[#24](https://github.com/anycable/anycable-go/issues/24)] No more ping logs even in debug mode.

## 0.5.4 (2018-02-08)

- Automatically reconnect to Redis when connection is lost. ([@palkan][])

Fixes [#25](https://github.com/anycable/anycable-go/issues/25).

## 0.5.3 (2017-12-22)

- Fix bug with non-JSON messages. ([@palkan][])

Fixes [#23](https://github.com/anycable/anycable-go/issues/23).

## 0.5.1 (2017-11-08)

- Add TLS support. ([@palkan][])

To secure your `anycable-go` server provide the paths to SSL certificate and private key:

```shell
anycable-go -addr=0.0.0.0:443 -ssl_cert=path/to/ssl.cert -ssl_key=path/to/ssl.key

=> Running AnyCable websocket server (secured) v0.5.1 on 0.0.0.0:443 at /cable
```

- Handle RPC errors gracefully. ([@palkan][])

Avoid panic when RPC server is unavailable. All RPC call now return `(response, error)`.

## 0.5.0 (2017-10-20)

- Support passing arbitrary headers to RPC. ([@palkan][])

Added new CLI option to pass a list of headers:

```sh
anycable-go -headers=cookie,x-api-token,origin
```

By default equals "cookie".

- Send control frame before closing connections. ([@palkan][])

## 0.4.2 (2017-09-27)

- Fixed bug with race conditions in hub. ([@palkan][])

Fixed [#10](https://github.com/anycable/anycable-go/issues/10).

## 0.4.0 (2017-03-18)

- Follow AnyCable versioning conventions. ([@palkan][])

Add `-version` flag to show current version.
Print current version on startup.

## 0.3.0 (2017-01-22)

- Refactor RPC methods. ([@palkan][])

Use one `Command` call instead of separate calls (`Subscribe`, `Unsubscribe`, `Perform`).

- Fix ping message format. ([@woodcrust][])

Do not add `identifier` field.

## 0.2.0 (2016-12-28)

- Add `DisconnectNotifier`. ([@palkan][])

`DisconnectNotifier` invokes RPC `Disconnect` _gracefully_, i.e. with the rate limit
(100 requests per second by default).

- Refactor `Pinger`. ([@palkan][])

`Pinger` now is always running and track the number of active connections by itself
(no need to call `hub.Size()`).
No more race conditions.

- Small fixes. ([@palkan][])

[@palkan]: https://github.com/palkan
[@woodcrust]: https://github.com/woodcrust
[@shedimon]: https://github.com/shedimon
[@sponomarev]: https://github.com/sponomarev
