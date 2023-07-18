# Change log

## master

- Add TLS support for gRPC server. ([@Envek][])

## 1.4.0 (2023-07-07)

- Add HTTP RPC support. ([@palkan][])

- Add `redisx` broadcast adapter. ([@palkan][])

A next-gen (Broker-compatible) broadcasting adapter using Redis Streams.

## 1.3.1 (2023-05-12)

- Fix gRPC health check response for an empty service. ([@palkan][])

As per [docs](https://github.com/grpc/grpc/blob/master/doc/health-checking.md), an empty service is used as the key for server's overall health status. So, if we serve any services, we should return `SERVING` status.

- Add gRPC health check when using grpc_kit. ([@palkan][])

## 1.3.0 (2023-02-28)

- Add configuration presets. ([@palkan][])

Provide sensible defaults matching current platform (e.g., Fly.io).

- Require Ruby 2.7+ and Anyway Config 2.2+.

- Add `rpc_max_connection_age` option (if favour of `rpc_server_args.max_connection_age_ms`) and configured its **default value to be 300 (5 minutes)**. ([@palkan][])

- (_Experimental_) Add support for [grpc_kit](https://github.com/cookpad/grpc_kit) as an alternative gRPC implementation. ([@palkan][], [@cylon-v][])

Add `grpc_kit` to your Gemfile and specify `ANYCABLE_GRPC_IMPL=grpc_kit` env var to use it.

- Setting the redis driver to ruby specified. ([@smasry][])

- Add mutual TLS support for connections to Redis. ([@Envek][])

  `ANYCABLE_REDIS_TLS_CLIENT_CERT_PATH` and `ANYCABLE_REDIS_TLS_CLIENT_KEY_PATH` settings to specify client certificate and key when connecting to Redis server that requires clients to authenticate themselves.

## 1.2.5 (2022-12-01)

- Add `ANYCABLE_REDIS_TLS_VERIFY` setting to disable validation of Redis server TLS certificate. ([@Envek][])

## 1.2.4 (2022-08-10)

- Add NATS pub/sub adapter. ([@palkan][])

- Setting the redis adapter to use the ruby driver. ([@smasry][])

## 1.2.3 (2022-04-20)

- Pass unique connection id (_session id_) in the `anycable.sid` Rack env field. ([@palkan][])

## 1.2.2 (2022-03-04)

- Allow Ruby 2.6.

## 1.2.1 (2022-02-21)

- Fix RBS signature. ([@palkan][])

- Add empty (`''`) service to gRPC health check as "NOT_SERVING". ([@palkan][])

## 1.2.0 (2021-12-21) 🎄

- Drop Ruby 2.6 support.

## 1.1.4 (2021-11-11)

- Do not swallow `grpc` missing .so exceptions. ([@palkan][])

## 1.1.3 (2021-09-29)

- Added support for type coercion from Anyway Config 2.2. ([@palkan][])

## 1.1.2 (2021-09-10) 🤵👰

- Improved gRPC server args support. ([@palkan][])

Add ability to declare gRPC server args without namespacing (i.e., `"max_connection_age_ms"` instead of `"grpc.max_connection_age_ms"`). That makes it possible to use ENV vars to provide the gRPC configuration.

## 1.1.1 (2021-06-05)

- Fixed error message when RPC implementation is missing. ([@palkan][])

We haven't extracted `anycable-grpc` yet.

## 1.1.0 🚸 (2021-06-01)

- No changes since 1.1.0.rc1.

## 1.1.0.rc1 (2021-05-12)

- **BREAKING** Move middlewares from gRPC interceptors to custom implementation. ([@palkan][])

That allowed us to have _real_ middlewares with ability to modify responses, intercept exceptions, etc.
The API changed a bit:

```diff
 class SomeMiddleware < AnyCable::Middleware
-  def call(request, rpc_call, rpc_handler)
+  def call(rpc_method_name, request, metadata)
     yield
   end
 end
```

- **Ruby >= 2.6** is required.
- **Anyway Config >= 2.1** is required.

## 1.0.3 (2021-03-05)

- Ruby 3.0 compatibility. ([@palkan][])

## 1.0.2 (2021-01-05)

- Handle TLS Redis connections by using VERIFY_NONE mode. ([@palkan][])

## 1.0.1 (2020-07-07)

- Support providing passwords for Redis Sentinels. ([@palkan][])

Use the following format: `ANYCABLE_REDIS_SENTINELS=:password1@my.redis.sentinel.first:26380,:password2@my.redis.sentinel.second:26380`.

## 1.0.0 (2020-07-01)

- Add `embedded` option to CLI runner. ([@palkan][])

- Add `Env#istate` and `EnvResponse#istate` to store channel state. ([@palkan][])

That would allow to mimic instance variables usage in Action Cable channels.

- Add `CommandResponse#stopped_streams` to support unsubscribing from particular broadcastings. ([@palkan])

`Socket#unsubscribe` is now implemented as well.

- Add `AnyCable.broadcast_adapter#broadcast_command` method. ([@palkan][])

It could be used to send commands to WS server (e.g., remote disconnect).

- Add `:http` broadcasting adapter. ([@palkan][])

- **RPC schema has changed**. ([@palkan][])

Using `anycable-go` v1.x is required.

- **Ruby 2.5+ is required**. ([@palkan][])

- Added RPC proto version check. ([@palkan][])

Server must sent `protov` metadata with the supported versions (comma-separated list). If there is no matching version an exception is raised.

Current RPC proto version is **v1**.

- Added `request` support to channels. ([@palkan][])

Now you can access `request` object in channels, too (e.g., to read headers/cookies/URL/etc).

- Change default server address from `[::]:50051` to `127.0.0.1:50051`. ([@palkan][])

See [#71](https://github.com/anycable/anycable/pull/71).

- Fix building Redis Sentinel config. ([@palkan][])

---

See [Changelog](https://github.com/anycable/anycable/blob/0-6-stable/CHANGELOG.md) for versions <1.0.0.

[@palkan]: https://github.com/palkan
[@smasry]: https://github.com/smasry
[@Envek]: https://github.com/Envek
[@cylon-v]: https://github.com/cylon-v
