# Change log

## master (0.6.0)

- **[Breaking Change]** New configuration and CLI options.

Environment variables now should be prefixed with `ANYCABLE_`.

`REDIS_URL` and `PORT` env variables are recognized by default.

Setting server address is split into two params: `--port` and `--host` (instead of `--addr`).

Run `anycable-go -h` to learn more.

- New logging format.

Now we use _structured_ logging with the help if [apex/log](https://github.com/apex/log). For example:

```
INFO 2018-03-05T08:44:57.684Z context=main Starting AnyCable unknown
INFO 2018-03-05T08:44:57.684Z context=main Handle WebSocket connections at /cable
INFO 2018-03-05T08:44:57.684Z context=http Starting HTTP server at 0.0.0.0:8080
INFO 2018-03-05T08:44:57.685Z context=rpc RPC pool initialized: 0.0.0.0:50051
INFO 2018-03-05T08:44:57.695Z context=pubsub Subscribed to Redis channel: __anycable__
```

Also, `json` format is supported out-of-the-box (`--log_format=json` or `ANYCABLE_LOG_FORMAT=json`).

- Add unique identifiers to connections.

Helps to identify connections in logs. Will be included into RPC calls in the future releases.

- [Closes[#24](https://github.com/anycable/anycable-go/issues/24)] No more ping logs even in debug mode.

## 0.5.4 (2018-02-08)

- Automatically reconnect to Redis when connection is lost. ([@palkan][])

Fixes [#25](https://github.com/anycable/anycable-go/issues/25).

## 0.5.3 (2017-12-22)

- Fix bug with non-JSON messages. ([@palkan][])

Fixes [#23](https://github.com/anycable/anycable-go/issues/23).

## 0.5.1 (2017-11-08)

- Add TLS support. ([@palkan][])

To secure your `anycable-go` server provide the paths to SSL certificate and private key:

```shell
anycable-go -addr=0.0.0.0:443 -ssl_cert=path/to/ssl.cert -ssl_key=path/to/ssl.key

=> Running AnyCable websocket server (secured) v0.5.1 on 0.0.0.0:443 at /cable
```

- Handle RPC errors gracefully. ([@palkan][])

Avoid panic when RPC server is unavailable. All RPC call now return `(response, error)`.

## 0.5.0 (2017-10-20)

- Support passing arbitrary headers to RPC. ([@palkan][])

Added new CLI option to pass a list of headers:

```sh
anycable-go -headers=cookie,x-api-token,origin
```

By default equals "cookie".

- Send control frame before closing connections. ([@palkan][])

## 0.4.2 (2017-09-27)

- Fixed bug with race conditions in hub. ([@palkan][])

Fixed [#10](https://github.com/anycable/anycable-go/issues/10).

## 0.4.0 (2017-03-18)

- Follow AnyCable versioning conventions. ([@palkan][])

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
