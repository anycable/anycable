# Change log

## master

## 1.1.0.rc1

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
[@sponomarev]: https://github.com/sponomarev
[@bibendi]: https://github.com/bibendi
