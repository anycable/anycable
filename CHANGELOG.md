# Change log

## master

– Add `REMOTE_ADDR` socket env variable using a synthetic header passed from a websocket
server. ([@sponomarev][])

Recreating a request object in your custom connection factory using `Rack::Request` or
`ActionDispatch::Request` (already implemented in [anycable-rails](https://github.com/anycable/anycable-rails))
gives you an access to `request.ip` with the properly set IP address.

- Align socket env to be more compatibile with Rack Spec ([@sponomarev][])

Provide as much env details as possible to be able to reconstruct the full
request object in a custom connection factory.

## 0.6.3 (2019-03-26)

- Relax `redis` gem version requirement. ([@palkan][])

Use the same restriction as Action Cable does (`>= 3`).

## 0.6.2 (2019-03-15)

- Add GRPC service method name and message content to exception notifications ([@sponomarev][])

`Anycable.capture_exception` allows accessing GRPC service method name and message content
on which an exception was captured. It can be used for exceptions grouping in your tracker and
providing additional data to investigate a root of a problem.

Example:

```ruby
AnyCable.capture_exception do |ex, method, message|
  Honeybadger.notify(ex, component: "any_cable", action: method, params: message)
end
```

Usage of a handler proc with just a single argument is preserved for the sake of compatibility.

- Add deprecation warning to default host usage ([@sponomarev][])

Exposing AnyCable publicly is considered to be harmful and planned to be changed
in future versions.

- Allow running the server as a detachable daemon ([@sponomarev][])

Server is fully managed by the binary itself.

```
# Start anycable daemon
$ bundle exec anycabled start

# Pass cli options to anycable through daemon. Separate daemon options and anycable options with `--`
$ bundle exec anycabled start -- --rpc-host 127.0.0.1:31337

# Stop anycable daemon
$ bundle exec anycabled stop

# See more anycable daemon options
$ bundle exec anycabled
```

## 0.6.1 (2019-01-05)

- [Fix #63](https://github.com/anycable/anycable-rails/issues/63) Load `anyway_config` after application boot to make sure that all frameworks dependent functionality is loaded. ([@palkan][])

## 0.6.0 (2018-11-15)

### Features

#### Broadcast adapters

AnyCable allows you to use custom broadcasting adapters (Redis is used by default):

```ruby
# Specify by name (tries to load `AnyCable::BroadcastAdapters::MyAdapter` from
# "anycable/broadcast_adapters/my_adapter")
AnyCable.broadcast_adapter = :my_adapter, { option: "value" }
# or provide an instance (should respond_to #broadcast)
AnyCable.broadcast_adapter = MyAdapter.new
```

**Breaking:** to use Redis adapter you must ensure that it is present in your Gemfile; AnyCable gem doesn't have `redis` as a dependency anymore.

#### CLI

AnyCable now ships with a CLI–`anycable`.

Use it to run a gRPC server:

```sh
# run anycable and load app from app.rb
bundle exec anycable -r app.rb
# or
bundle exec anycable --require app.rb
```

All configuration options are also supported as CLI options (see `anycable -h` for more information).

The only required options is the application file to load (`-r/--require`).

You can omit it if you want to load an app form `./config/environment.rb` (e.g. with Rails) or `./config/anycable.rb`.

AnyCable CLI also allows you to run a separate command (process) from within a RPC server:

```sh
$ bundle exec anycable --server-command "anycable-go -p 3334"
```

#### Configuration

- Default server host is changed from `localhost:50051` to `0.0.0.0:50051`
- Expose gRPC server parameters via `rpc_*` config params:

```ruby
AnyCable.configure do |config|
  config.rpc_pool_size = 120
  config.rpc_max_waiting_requests = 10
  # etc
end
```
- `REDIS_URL` env is used by default if present (and no `ANYCABLE_REDIS_URL` specified)
- Make HTTP health check url configurable
- Add ability to pass Redis Sentinel config as array of string.

Now it's possible to pass Sentinel configuration via env vars:

```sh
ANYCABLE_REDIS_SENTINELS=127.0.0.1:26380,127.0.0.1:26381 bundle exec anycable
```

#### Other

- Added middlewares support

See [docs](https://docs.anycable.io/#/./middlewares).

- Added gRPC health checker.

See [docs](https://docs.anycable.io/#/./health_checking).

- Added hook to run code only within RPC server context.

Use `AnyCable.configure_server { ... }` to run code only when RPC server is running.

### API changes

**NOTE**: the old API is still working but deprecated (you'll see a notice).

- Use `AnyCable` instead of `Anycable`

- New API for registering error handlers:

```ruby
AnyCable.capture_exception do |ex|
  Honeybadger.notify(ex)
end
```

- `AnyCable::Server.start` is deprecated


## 0.5.2 (2018-09-06)

- [#48](https://github.com/anycable/anycable/pull/48) Add HTTP health server ([@DarthSim][])

## 0.5.1 (2018-06-13)

Minor fixes.

## 0.5.0 (2017-10-21)

- [#2](https://github.com/anycable/anycable/issues/2) Add support for [Redis Sentinel](https://redis.io/topics/sentinel). ([@accessd][])

- [#28](https://github.com/anycable/anycable/issues/28) Support arbitrary headers. ([@palkan][])

Previously we hardcoded only "Cookie" header. Now we add all passed headers by WebSocket server to request env.

- [#27](https://github.com/anycable/anycable/issues/27) Add `error_msg` to RPC responses. ([@palkan][])

Now RPC responses has 3 statuses:

  - `SUCCESS` – successful request, operation succeed
  - `FAILURE` – successful request, operation failed (e.g. authentication failed)
  - `ERROR` – request failed (exception raised).

We provide `error_msg` only when request status is `ERROR`.

- [#25](https://github.com/anycable/anycable/issues/25) Improve logging and exceptions handling. ([@palkan][])

Default logger logs to STDOUT with `info` level by default but can be configured to log to file with
any severity.

GRPC logging is turned off by default (can be turned on through `log_grpc` configuration parameter).

`ANYCABLE_DEBUG=1` acts as a shortcut to set `debug` level and turn on GRPC logging.

Now it's possible to add custom exception handlers (e.g. to notify external exception tracking services).

More on [Wiki](https://github.com/anycable/anycable/wiki/Logging-&-Exceptions-Handling).

## 0.4.6 (2017-05-20)

- Add `Anycable::Server#stop` method. ([@sadovnik][])

## 0.4.5 (2017-03-17)

- Fixed #11. ([@palkan][])

## 0.4.4 (2017-03-06)

- Handle `StandardError` gracefully in RPC calls. ([@palkan][])

## 0.4.3 (2017-02-18)

- Update `grpc` version dependency to support Ruby 2.4. ([@palkan][])

## 0.4.2 (2017-01-28)

- Change socket streaming API. ([@palkan][])

Add `Socket#subscribe`, `unsubscribe` and `unsubscribe_from_all` methods.

## 0.4.1 (2017-01-24)

- Introduce _fake_ socket instance to handle transmissions and streams. ([@palkan][])

- Make commands handling more abstract. ([@palkan][])

We now do not explicitly call channels action but use the only one entrypoing for all commands:

```ruby
connection.handle_channel_command(identifier, command, data)
```

This method should return `true` if command was successful and `false` otherwise.

## 0.4.0 (2017-01-22)

- Refactor RPC API. ([@palkan][])

Replace `Subscribe`, `Unsubscribe` and `Perform` methods with `Command` method.

- Extract Rails functionality to separate gem. ([@palkan][])

All Rails specifics now live here https://github.com/anycable/anycable-rails.

## 0.3.0 (2016-12-28)

- Handle `Disconnect` requests. ([@palkan][])

Implement `Disconnect` handler, which invokes `Connection#disconnect` (along with `Channel#unsubscribed` for each subscription).

[@palkan]: https://github.com/palkan
[@sadovnik]: https://github.com/sadovnik
[@accessd]: https://github.com/accessd
[@DarthSim]: https://github.com/DarthSim
[@sponomarev]: https://github.com/sponomarev
