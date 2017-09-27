# Change log

## 0.4.2 (2017-09-27)

- Fixed bug with race conditions in hub. ([@palkan][])

Fixed [#10](https://github.com/anycable/anycable-go/issues/10).

## 0.4.0 (2017-03-18)

- Follow AnyCable versioning conventions. ([@palkan]())

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
