# Change log (Pro version)

## master

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
