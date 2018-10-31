# Change log

## master

## 0.6.0-dev

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
