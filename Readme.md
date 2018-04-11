[![Build Status](https://travis-ci.org/anycable/anycable-go.svg?branch=master)](https://travis-ci.org/anycable/anycable-go) [![Dependency Status](https://dependencyci.com/github/anycable/anycable-go/badge)](https://dependencyci.com/github/anycable/anycable-go) [![Gitter](https://img.shields.io/badge/gitter-join%20chat%20%E2%86%92-brightgreen.svg)](https://gitter.im/anycable/anycable-go)

# AnyCable-Go WebSocket Server

WebSocket server for [AnyCable](https://github.com/anycable/anycable).

**NOTE:** this is a readme for the upcoming 0.6.0 version. [Go to 0.5.x version](https://github.com/anycable/anycable-go/tree/0-5-stable).

## Installation

The easiest way to install AnyCable-Go is to [download](https://github.com/anycable/anycable-go/releases) a pre-compiled binary.

Or with [Homebrew](https://brew.sh/)

```shell
brew install anycable/anycable/anycable-go
```

Of course, you can install it from source too:

```shell
go get -u -f github.com/anycable/anycable-go/cmd/anycable-go
```

## Upgrade

For instructions on how to upgrade to a newer version see [upgrade notes](https://github.com/anycable/anycable-go/blob/master/UPGRADE.md).

### Heroku

See [heroku-anycable-go](https://github.com/anycable/heroku-anycable-go) buildpack.

## Usage

Run server:

```shell
anycable-go --rpc_host=0.0.0.0:50051 --headers=cookie,x-api-token --redis_url=redis://localhost:6379/5 --redis_channel=anycable --host=0.0.0.0 --port=8080

=> INFO 2018-03-05T08:44:57.684Z context=main Starting AnyCable 0.6.0
```

You can also provide configuration parameters through the corresponding environment variables (i.e. `ANYCABLE_RPC_HOST`, `ANYCABLE_REDIS_URL`, etc).

For more information about available options run `anycable-go -h`.

## Metrics & Stats

Anycable-Go provides useful statistical information about the service (such as a number of connected clients, received messages, etc.).

### Prometheus

To enable a HTTP endpoint to serve [Prometheus](https://prometheus.io)-compatible metrics (disabled by default) you must specify `--metrics_http` option (e.g. `--metrics_http="/metrics"`).

You can also change a listening port and listening host through `--metrics_port` and `--metrics_host` options respectively (by default the same as the main (websocket) server port and host, i.e. using the same server).

The exported metrics format is the following:

```
# HELP anycable_go_clients_num The number of active clients
# TYPE anycable_go_clients_num gauge
anycable_go_clients_num 0

# HELP anycable_go_clients_uniq_num The number of unique clients (with respect to connection identifiers)
# TYPE anycable_go_clients_uniq_num gauge
anycable_go_clients_uniq_num 0

# HELP anycable_go_client_msg_total The total number of received messages from clients
# TYPE anycable_go_client_msg_total counter
anycable_go_client_msg_total 5906

# HELP anycable_go_failed_client_msg_total The total number of unrecognized messages received from clients
# TYPE anycable_go_failed_client_msg_total counter
anycable_go_failed_client_msg_total 0

# HELP anycable_go_broadcast_msg_total The total number of messages received through PubSub (for broadcast)
# TYPE anycable_go_broadcast_msg_total counter
anycable_go_broadcast_msg_total 956

# HELP anycable_go_failed_broadcast_msg_total The total number of unrecognized messages received through PubSub
# TYPE anycable_go_failed_broadcast_msg_total counter
anycable_go_failed_broadcast_msg_total 0

# HELP anycable_go_broadcast_streams_total The number of active broadcasting streams
# TYPE anycable_go_broadcast_streams_total gauge
anycable_go_broadcast_streams_total 0

# HELP anycable_go_rpc_call_total The total number of RPC calls
# TYPE anycable_go_rpc_call_total counter
anycable_go_rpc_call_total 15808

# HELP anycable_go_rpc_error_total The total number of failed RPC calls
# TYPE anycable_go_rpc_error_total counter
anycable_go_rpc_error_total 0

# HELP anycable_go_failed_auths_total The total number of failed authentication attempts
# TYPE anycable_go_failed_auths_total counter
anycable_go_failed_auths_total 0

# HELP anycable_go_goroutines_num The number of Go routines
# TYPE anycable_go_goroutines_num gauge
anycable_go_goroutines_num 5222

# HELP anycable_go_disconnect_queue_size The size of delayed disconnect
# TYPE anycable_go_disconnect_queue_size gauge
anycable_go_disconnect_queue_size 0
```

### Logging

Another option is to periodically write stats to log (with `info` level).
To enable metrics logging pass `--metrics_log` flag.

Your logs should contain smth like:

```
INFO 2018-03-06T14:16:27.872Z broadcast_msg_total=0 broadcast_streams_num=0 client_msg_total=0 clients_num=0 clients_uniq_num=0 context=metrics disconnect_queue_size=0 failed_auths_total=0 failed_broadcast_msg_total=0 failed_client_msg_total=0 goroutines_num=35 rpc_call_total=0 rpc_error_total=0
```

By default metrics are logged every 15 seconds (you can change this behaviour through `--metrics_log_interval` option).

## Throubleshooting

First, try to run `anycable-go --debug` to enable verbose logging.

The most common problem is using different Redis channels within RPC instance and `anycable-go`. Find the following line in the logs:

```
INFO 2018-03-05T08:44:57.695Z context=pubsub Subscribed to Redis channel: __anycable__
```

and make sure, that RPC server publishes messages to the specified channel.

### TLS

To secure your `anycable-go` server provide the paths to SSL certificate and private key:

```shell
anycable-go --port=443 -ssl_cert=path/to/ssl.cert -ssl_key=path/to/ssl.key

=> INFO 2018-03-05T08:44:57.684Z context=http Starting HTTPS server at 0.0.0.0:443
```

## Build

```shell
make
```

## Docker

See available images [here](https://hub.docker.com/r/anycable/anycable-go/).

## ActionCable Compatibility

Feature                  | Status
-------------------------|--------
Connection Identifiers   | +
Connection Request (cookies, params) | +
Disconnect Handling | +
Subscribe to channels | +
Parameterized subscriptions | +
Unsubscribe from channels | +
Performing Channel Actions | +
Streaming | +
Usage of the same stream name for different channels | +
Broadcasting | +
Remote disconnect | - (WIP)
[Custom stream callbacks](http://edgeapi.rubyonrails.org/classes/ActionCable/Channel/Streams.html) | -
[Subscription Instance Variables](http://edgeapi.rubyonrails.org/classes/ActionCable/Channel/Streams.html) | -

## Contributing

Bug reports and pull requests are welcome on GitHub at https://github.com/anycable/anycable-go.

## License
The library is available as open source under the terms of the [MIT License](http://opensource.org/licenses/MIT).
