# Change log

## master

## 1.5.3 (2024-08-28)

- Fix potential deadlocks in Redis pub/sub on reconnect. ([@palkan][])

## 1.5.2 (2024-06-04)

- Add `?raw=1` option for EventSource connections to receive only data messages (no protocol messages). ([@palkan][])

## 1.5.1 (2024-04-16)

- Add `?history_since` support for SSE connections. ([@palkan][])

- Add `?stream` and `?signed_stream` support for SSE connections. ([@palkan][])

## 1.5.0 (2024-04-01)

- Add _whispering_ support to pub/sub streams. ([@palkan][])

Whispering is an ability for client to publish events to the subscribed stream without involving any server-side logic. Whenever a client _whispers_ an event to the channel, it's broadcasted to all other connected clients.

The feature is enabled for public streams automatically. For signed streams, you must opt-in via the `--streams_whisper` (`ANYCABLE_STREAMS_WHISPER=true`) option.

- HTTP broadcaster is enabled by default. ([@palkan][])

HTTP broadcaster is now enabled by default in addition to Redis (backward-compatibility). It will be the primary default broadcaster in v2.

- Added new top-level `--secret` configuration option. ([@palkan][])

This is a one secret to rule them all! Unless specific secrets are specified, all features relying on secret keys use the global (_application_) secret either directly (signed streams, JWT) or as a secret base to generate auth tokens (HTTP broadcasting and HTTP RPC).

- Added public mode (`--public`). ([@palkan][])

In this mode, AnyCable doesn't enforce connection authentication, enables public streams, and disables HTTP broadcast endpoint authentication. (So, it's a combination of `--noauth` and `--public_streams`).

- Added direct stream subscribing via a dedicated channel. ([@palkan][])

Added a `$pubsub` reserved channel to allow subscribing to streams (signed or unsigned) without relying on channels (similar to Turbo Streams).

Enable public (unsigned) streams support via the `--public_streams` (or `ANYCABLE_PUBLIC_STREAMS=1`) option or provide a secret key to verify signed streams via the `--streams_secret=<val>` (or `ANYCABLE_STREAMS_SECRET=<val>`) option.

- Config options refactoring (renaming):

  - Deprecated (but still supported): `--turbo_rails_key` (now `--turbo_streams_secret`), `--cable_ready_key` (now `--cable_ready_secret`), `--jwt_id_key` (now `--jwt_secret`), `--jwt_id_param` (now `--jwt_param`), `--jwt_id_enforce` (now `--enforce_jwt`).
  - Deprecated (no longer supported): `--turbo_rails_cleartext`, `--cable_ready_cleartext` (use `--public_streams` instead).

- Allowing embedding AnyCable into existing web applications. ([@palkan][])

You can now set up an AnyCable instance without an HTTP server and mount AnyCable WebSocket/SSE handlers wherever you like.

- Log format has changed. ([@palkan][])

We've migrated to Go `slog` package and its default text handler format.

## 1.4.8 (2024-01-10)

- Add `--redis_disable_cache` option to disable client-side caching (not supported by some Redis providers). ([@palkan][])

- Return support for an scheme omitted IP address in RPC_HOST. ([@ardecvz][])

For example, `127.0.0.1:50051/anycable`.

It's missing from 1.4.6 which introduced auto RPC implementation inferring from the RPC host.

## 1.4.7 (2023-11-03)

- Add NATS-based broker. ([@palkan][])

## 1.4.6 (2023-10-25)

- Add `@anycable/anycable-go` NPM package to install `anycable-go` as a JS project dependency. ([@palkan][])

- Infer RPC implementation from RPC host. ([@palkan][])

Feel free to drop `rpc_impl` configuration parameter.

- Enhance Fly configuration preset to enable embedded NATS and configure HTTP broadcaster port. ([@palkan][])

## 1.4.5 (2023-10-15)

- Fix passing RPC context via headers when using HTTP RPC. ([@palkan][])

- Add `exclude_socket` option support to broadcasts. ([@palkan][])

- Add support for batched broadcasts. ([@palkan][])

It's now possible to publish an array of broadcasting messages (e.g., `[{"stream":"a","data":"..."},"stream":"b","data":"..."}]`). The messages will be delivered in the same order as they were published (within a stream).

- Add support for REDIS_URL with trailing slash hostnames. ([@ardecvz][])

`rueidis` doesn't tolerate the trailing slash hostnames.

- Support to use a custom database number. ([@ardecvz][])

Parse `--redis_url` (`ANYCABLE_REDIS_URL` or `REDIS_URL`) using the `rueidis` standard utility.

## 1.4.4 (2023-09-07)

- Add Server-Sent Events support. ([@palkan][])

- Add `disconnect_backlog_size` option to control the buffer size for the disconnect queue.

## 1.4.3 (2023-08-10)

- Added ability to require `pong` messages from clients (for better detection of broken connections). ([@palkan][])

Use `--pong_timoout=<number of seconds to wait for ping>` option to enable this feature. **NOTE:** This option is only applied to clients using the extended Action Cable protocol.

- Allow configuring ping interval and ping timestamp precision per connection. ([@palkan][])

You can use URL query params to configure ping interval and timestamp precision per connection:

For example, using the following URL, you can set the ping interval to 10 seconds and the timestamp precision to milliseconds:

```txt
ws://localhost:8080/cable?pi=10&ptp=ms
```

- Fix in-memory streams history retrieval. ([@palkan][])

Keep empty streams in the cache for a longer time, so we re-use the offset. In case the stream has been disposed, we should return an error when the requested offset is out of range.

## 1.4.2 (2023-08-04)

- Add `--shutdown_timeout` option to configure graceful shutdown timeout. ([@palkan][])

This new option deprecates the older `--disconnect_timeout` option (which now has no effect).

## 1.4.1 (2023-07-24)

- Allow to configure secure gRPC connection. ([@Envek][])

  Option `--rpc_tls_root_ca` allow to specify private root certificate authority certificate to verify RPC server certificate.
  Option `--rpc_tls_verify` allow to disable RPC server certificate verification (insecure, use only in test/development).

## 1.4.0 (2023-07-07)

- Add HTTP RPC implementation. ([@palkan][])

- Added telemetry data collection. ([@palkan][])

- Add `--turbo_rails_cleartext` and `--cable_ready_cleartext`. ([@palkan][])

In a clear-text mode, stream names are passed as-is from the client, no signing is required. This mode is meant for experimenting with Turbo Streams or Cable Ready streams in non-Rails applications.

- Introduce `--disconnect_mode` and deprecate `--disable_disconnect`. ([@palkan][])

You can use `--disconnect_mode=never` to achieve the same behaviour as with `--disable_disconnect`. The new (and default) `--disconnect_mode=auto` automatically detects if a Disconnect call is needed for a session (for example, Hotwire clients don't need it).

- Refactor broadcasting to preserve the order of messages within a single stream. ([@palkan][])

Previously, we used a Go routine pool to concurrently delivery broadcasts, which led to nondeterministic order of messages within a single stream delivered in a short period of time. Now, we preserve the order of messages within a stream—the delivered as they were accepted by the server.

**NOTE:** Keep in mind that in a clustered setup with a non fan-out broadcaster (i.e., when using a broker), each broadcasted message is delivered to only a single node in the cluster (picked randomly). In this case, we cannot guarantee the resulting order.

- Add `redisx` broadcast adapter. ([@palkan][])

- Set HTTP broadcast to `$PORT` on Heroku by default. ([@palkan][])

- Add `broker` preset to configure required components to use an in-memory broker. ([@palkan][])

- New broadcasting architecture: broadcasters, subscribers and brokers. ([@palkan][])

## 1.3.1 (2023-03-22)

- Add `--gateway_advertise` option for embedded NATS. ([@palkan][])

## 1.3.0 (2023-02-28)

- Add configuration presets (fly, heroku). ([@palkan][])

Provide sensible defaults matching current platform.

- Add embedded NATS support. ([@gzigzigzeo][], [@palkan][])

- Added metrics default tags support for StatsD and Prometheus.

You can define global tags (added to every reported metric by default) for Prometheus (reported as labels)
and StatsD. For example, we can add environment and node information:

```sh
anycable-go --metrics_tags=environment:production,node_id:xyz
# or via environment variables
ANYCABLE_METRICS_TAGS=environment:production,node_id:xyz anycable-go
```

For StatsD, you can specify tags format: "datadog" (default), "influxdb", or "graphite".
Use the `statsd_tag_format` configuration parameter for that.

- Added Statsd support.

You can send instrumentation data to Statsd.
Specify `statsd_host` and (optionally) `statsd_prefix`:

```sh
anycable-go -statsd_host=localhost:8125 -statsd_prefix=anycable.
```

- Add `grpc_active_conn_num` metrics. ([@palkan][])

Useful when you use DNS load balancing to know, how many active gRPC connections are established.

Also, added debug logs for gRPC connected/disconnected events.

- Support placeholders and wildcards in `--path`. ([@palkan][])

## 1.2.3 (2022-12-01)

- Add `redis_tls_verify` setting to enable validation of Redis server TLS certificate. ([@Envek][])

- Add `--proxy-cookies` setting to filter cookies passed to RPC. ([@rafaelrubbioli][])

## 1.2.2 (2022-08-10)

- Add NATS pub/sub adapter. ([@palkan][])

## 1.2.1 (2022-06-30)

- Use [automaxprocs](https://github.com/uber-go/automaxprocs) to configure the number of OS threads. ([@palkan][])

- Fix race conditions when stopping and starting streams within the same command. ([@palkan][])

## 1.2.0 (2021-12-21) 🎄

- Add fastlane subscribing for Hotwire (Turbo Streams) and CableReady.

Make it possible to terminate subscription requests at AnyCable Go without performing RPC calls.

- Add JWT authentication/identification support.

You can pass a properly structured token along the connection request to authorize the connection and set up _identifiers_ without performing an RPC call.

## 1.1.4 (2021-11-16)

- Add `rpc_max_call_recv_size` and `rpc_max_call_send_size` options to allow modifying the corresponding limits for gRPC client connection. ([@palkan][])

## 1.1.3 (2021-09-16)

- Fixed potential deadlocks in Hub. ([@palkan][])

We noticed that Hub could become unresponsive due to a deadlock on `streamsMu` under a very high load,
so we make locking more granular and removed _nested_ locking.

## 1.1.2 (2021-06-23)

- Added `--rpc_enable_tls` option. ([@ryansch][])

- Do not treat RPC failures as errors. ([@palkan][])

Failure (e.g., subscription rejection) is an expected application behaviour and
should not be treated as error (e.g., logged).

## 1.1.1 (2021-06-15)

- Fixed potential concurrent read/write in hub. ([@palkan][])

## 1.1.0 🚸 (2021-06-01)

- Added `--max-conn` option to limit simultaneous server connections. ([@skryukov][])

- Added `data_sent_bytes_total` and `data_rcvd_bytes_total` metrics. ([@palkan][])

- Add `--allowed_origins` option to enable Origin check during the WebSocket upgrade. ([@skryukov][])

- Renamed `metrics_log_interval` to `metrics_rotate_interval`. ([@palkan][])

## 1.1.0.rc1 (2021-05-12)

- Added `server_msg_total` and `failed_server_msg_total` metrics. ([@prburgu][])

- Dropped deprecated RPC v0.6 support. ([@palkan][])

- Made ping message timestamp precision configurable ([@prburgu][])

- Added concurrency to broadcasting. ([@palkan][])

Now new broadcast messages are handled (and re-transmitted) concurrently by a pool of workers (Go routines).
You can control the size of the pool via the `hub_gopool_size` configuration parameter (defaults to 16).

## 1.0.5 (2021-03-17)

- Fix interval values for counters. ([@prburgu][])

See [#128](https://github.com/anycable/anycable-go/pull/128).

- Make ping and stats refresh intervals configurable. ([@palkan][])

Added `--ping_interval N` and `--stats_refresh_interval N` options respectively (both use seconds).

## 1.0.4 (2021-03-04)

- Fix race conditions in Hub. ([@palkan][])

Use a single channel for register/unregister and subscribe/unsubscribe to make order of
execution deterministic. Since `select .. case` chooses channels randomly, we may hit the situation when registration is added
after disconnection (_un-registration_).

- Add `sid=xxx` to RPC logs. ([@palkan][])

## 1.0.3 (2021-01-05)

- Handle TLS Redis connections by using VERIFY_NONE mode. ([@palkan][])

- Added `rpc_pending_num` metric. ([@palkan][])

## 1.0.2 (2020-09-08)

- Add channel states to `disconnect` requests. ([@palkan][])

- Moved pingMessage (session), disconnectMessage (node) and Reply (hub) structs into common package. ([@gr8bit][])

- Re-added git ref version to `LD_FLAGS` in Makefile. ([@gr8bit][])

## 1.0.1 (2020-07-07)

- Fix subscribing to the same stream from different channels. ([@palkan][])

- Support providing passwords for Redis Sentinels. ([@palkan][])

Use the following format: `ANYCABLE_REDIS_SENTINELS=:password1@my.redis.sentinel.first:26380,:password2@my.redis.sentinel.second:26380`.

- Fix setting `--metrics_host`. ([@palkan][])

See [#107](https://github.com/anycable/anycable-go/issues/107).

## 1.0.0 (2020-06-24)

- Add `--disable_disconnect` option. ([@palkan][])

Allows you to avoid calling `Disconnect` RPC method completely if you don't need it.

- Add channel state support. ([@palkan][])

- Add stopped streams support. ([@palkan][])

- Add support for remote commands. ([@palkan][])

Handle remote commands sent via Pub/Sub. Currently, only remote disconnect is supported.

- Add HTTP broadcasting adapter. ([@palkan][])

- Add Redis Sentinel support. ([@rolandg][])

- Send `disconnect` messages on server restart and authentication failures. ([@palkan][])

- Add `protov` RPC metadata. ([@palkan][])

- Add `rpc_retries_total` metrics. ([@palkan][])

This metrics represents the number of times RPC requests were retried.
The large value might indicate that the RPC server pool size doesn't correspond to the `rpc_concurrency` value.

- Use single gRPC client instance instead of a pool. ([@palkan][])

gRPC connection provides concurrency via H2 streams (with load balancing). Using a pool doesn't bring any performance
improvements and sometimes 'cause instability (e.g., ResourceExhausted or Unavailable exceptions under the load)

We still limit the number of concurrent RPC requests. Now you can configure it via `--rpc_concurrency` setting.

See [PR#88](https://github.com/anycable/anycable-go/pull/88) for more.

- Add `--disconnect_timeout` option to specify the timeout for graceful shutdown of the disconnect queue. ([@palkan][])

- Add `mem_sys_bytes` metric. ([@palkan][])

Returns the total bytes of memory obtained from the OS
(according to [`runtime.MemStats.Sys`](https://golang.org/pkg/runtime/#MemStats)).

- Add `--enable_ws_compression` option to enable WebSocket per message compression. ([@palkan][])

Disabled by default due to the experimental status in [Gorilla](https://github.com/gorilla/websocket/blob/c3e18be99d19e6b3e8f1559eea2c161a665c4b6b/doc.go#L201-L214).

- **IMPORTANT**: Docker images versioning changed from `vX.Y.Z` to `X.Y.Z`. ([@bibendi][])

Now you can specify only the part of the version, e.g. `anycable-go:1.0` instead of the full `anycable-go:v1.0.0`.

See [Changelog](https://github.com/anycable/anycable-go/blob/0-6-stable/CHANGELOG.md) for versions <1.0.0.

[@palkan]: https://github.com/palkan
[@sponomarev]: https://github.com/sponomarev
[@bibendi]: https://github.com/bibendi
[@rolandg]: https://github.com/rolandg
[@gr8bit]: https://github.com/gr8bit
[@prburgu]: https://github.com/prburgu
[@skryukov]: https://github.com/skryukov
[@ryansch]: https://github.com/ryansch
[@Envek]: https://github.com/Envek
[@rafaelrubbioli]: https://github.com/rafaelrubbioli
[@gzigzigzeo]: https://github.com/gzigzigzeo
[@ardecvz]: https://github.com/ardecvz
