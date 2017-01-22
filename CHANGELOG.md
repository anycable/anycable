# Change log

## 0.4.0

- Refactor RPC API. ([@palkan][])

Replace `Subscribe`, `Unsubscribe` and `Perform` methods with `Command` method.

- Extract Rails functionality to separate gem. ([@palkan][])

All Rails specifics now live here https://github.com/anycable/anycable-rails.

## 0.3.0 (2016-12-28)

- Handle `Disconnect` requests. ([@palkan][])

Implement `Disconnect` handler, which invokes `Connection#disconnect` (along with `Channel#unsubscribed` for each subscription).

[@palkan]: https://github.com/palkan
