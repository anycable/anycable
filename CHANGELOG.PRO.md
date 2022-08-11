# Change log (Pro version)

## master

## 1.2.2 (2022-12-01)

- Sync with OSS [v1.2.3](https://github.com/anycable/anycable-go/releases/tag/v1.2.3).

## 1.2.1 (2022-08-10)

- Add `graphql-ws` protocol support for GraphQL clients. ([@palkan][])

- Disconnect idle Apollo clients if no connection_init has been sent in the specified period of time.

- Support passing JWT token with connection params for Apollo clients.

## 1.2.0 (2022-06-30)

- Upgrade to Go 1.18.

Dependencies upgrades, minor fixes.

## 1.1.2

- Added metrics default tags support for StatsD and Prometheus.

You can define global tags (added to every reported metric by default) for Prometheus (reported as labels)
and StatsD. For example, we can add environment and node information:

```sh
anycable-go --metrics_tags=environment:production,node_id:xyz
# or via environment variables
ANYCABLE_METRICS_TAGS=environment:production,node_id:xyz anycable-go
```

For StatsD, you can specify tags format: "datadog" (default), "influxdb", or "graphite".
Use the `statsd_tag_format` configuration parameter for that.

## 1.1.1

- Added `graphql-ws` subprotocol to the list of supported procotols.

Fixes Apollo integration compatibility issues.

## 1.1.0

_No changes_

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
