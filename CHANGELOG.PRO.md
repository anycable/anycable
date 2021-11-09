# Change log (Pro version)

## master

## 1.1.1

- Added `graphql-ws` subprotocol to the list of supported procotols.

Fixes Apollo integration compatibility issues.

## 1.1.0

_No changes_

## 1.1.0.beta.2

- Add fastlane subscribing for Hotwire (Turbo Streams) and CableReady.

Make it possible to terminate subscription requests at AnyCable Go without performing RPC calls.

- Add JWT authentication/identification support.

You can pass a properly structured token along the connection request to authorize the connection and set up _identifiers_ without peforming an RPC call.

## 1.1.0.beta.1

- Refactored sessions to use Go pools and epoll/kqueue for messaging.

- Added Protobuf encoding support.

- Added Msgpack encoding support.

Use `"actioncable-v1-msgpack"` subprocol for the client connection to send and receive Msgpack encoded
data.

- Added Statsd support.

You can send instrumentation data to Statsd.
Specify `statsd_host` and (optionally) `statsd_prefix`:

```sh
anycable-go -statsd_host=localhost:8125 -statsd_prefix=anycable.
```

- Added Apollo GraphQL protocol support.
