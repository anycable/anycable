# Change log

## 0.5.0 (master)

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