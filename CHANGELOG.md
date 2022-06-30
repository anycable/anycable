# Change log

## master

- Use [automaxprocs](https://github.com/uber-go/automaxprocs) to configure the number of OS threads. ([@palkan][])

- Fix race conditions when stopping and starting streams within the same command. ([@palkan][])

## 1.2.0 (2021-12-21) 🎄

- Add fastlane subscribing for Hotwire (Turbo Streams) and CableReady.

Make it possible to terminate subscription requests at AnyCable Go without performing RPC calls.

- Add JWT authentication/identification support.

You can pass a properly structured token along the connection request to authorize the connection and set up _identifiers_ without peforming an RPC call.

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

Now new broadcast messages are handled (and retransmitted) concurrently by a pool of workers (Go routines).
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
improvements and sometimes 'cause unstability (e.g., ResourceExhausted or Unavailable exceptions under the load).

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
