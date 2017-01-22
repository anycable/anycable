# Change log

## 0.4.0

- Refactor RPC API. ([@palkan][])

- Extract Rails functionality to separate gem. ([@palkan][])

Replace `Subscribe`, `Unsubscribe` and `Perform` methods with `Command` method.

## 0.3.0 (2016-12-28)

- Handle `Disconnect` requests. ([@palkan][])

Implement `Disconnect` handler, which invokes `Connection#disconnect` (along with `Channel#unsubscribed` for each subscription).

[@palkan]: https://github.com/palkan
