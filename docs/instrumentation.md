# AnyCable-Go Instrumentation

AnyCable-Go provides useful statistical information about the service (such as the number of connected clients, received messages, etc.).

<p style="text-align:center;">
  <img width="70%" alt="AnyCable Grafana" src="/assets/images/grafana.png">
</p>

> Read the ["Real-time stress: AnyCable, k6, WebSockets, and Yabeda"](https://evilmartians.com/chronicles/real-time-stress-anycable-k6-websockets-and-yabeda) post to learn more about AnyCable observability and see example Grafana dashboards.

## Prometheus

To enable a HTTP endpoint to serve [Prometheus](https://prometheus.io)-compatible metrics (disabled by default) you must specify `--metrics_http` option (e.g. `--metrics_http="/metrics"`).

You can also change a listening port and listening host through `--metrics_port` and `--metrics_host` options respectively (by default the same as the main (websocket) server port and host, i.e., using the same server).

The exported metrics format is the following:

```sh
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

# HELP anycable_go_rpc_retries_total The total number of RPC call retries
# TYPE anycable_go_rpc_retries_total counter
anycable_go_rpc_retries_total 0

# HELP anycable_go_rpc_pending_num The number of pending RPC calls
# TYPE anycable_go_rpc_pending_num gauge
anycable_go_rpc_pending_num 0

# HELP anycable_go_failed_auths_total The total number of failed authentication attempts
# TYPE anycable_go_failed_auths_total counter
anycable_go_failed_auths_total 0

# HELP anycable_go_goroutines_num The number of Go routines
# TYPE anycable_go_goroutines_num gauge
anycable_go_goroutines_num 5222

# HELP anycable_go_disconnect_queue_size The size of delayed disconnect
# TYPE anycable_go_disconnect_queue_size gauge
anycable_go_disconnect_queue_size 0

# HELP anycable_go_server_msg_total The total number of messages sent to clients
# TYPE anycable_go_server_msg_total counter
anycable_go_server_msg_total 453

# HELP anycable_go_failed_server_msg_total The total number of messages failed to send to clients
# TYPE anycable_go_failed_server_msg_total counter
anycable_go_failed_server_msg_total 0

# HELP anycable_go_data_sent_total The total amount of bytes sent to clients
# TYPE anycable_go_data_sent_total counter
anycable_go_data_sent_total 1232434334

# HELP anycable_go_data_rcvd_total The total amount of bytes received from clients
# TYPE anycable_go_data_rcvd_total counter
anycable_go_data_rcvd_total 434334
```

<h2 id="statsd">StatsD <img class='pro-badge' src='https://docs.anycable.io/assets/pro.svg' alt='pro' /></h2>

AnyCable Pro also supports emitting real-time metrics to [StatsD](https://github.com/statsd/statsd).

For that, you must specify the StatsD server UDP host:

```sh
anycable-go -statsd_host=localhost:8125
```

Metrics are pushed with the `anycable_go.` prefix by default. You can override it by specifying the `statsd_prefix` parameter.

<h2 id="metrics-tags">Default metrics tags <img class='pro-badge' src='https://docs.anycable.io/assets/pro.svg' alt='pro' /></h2>

You can define global tags (added to every reported metric by default) for Prometheus (reported as labels)
and StatsD. For example, we can add environment and node information:

```sh
anycable-go --metrics_tags=environment:production,node_id:xyz
# or via environment variables
ANYCABLE_METRICS_TAGS=environment:production,node_id:xyz anycable-go
```

For StatsD, you can specify tags format: "datadog" (default), "influxdb", or "graphite".
Use the `statsd_tag_format` configuration parameter for that.

## Logging

Another option is to periodically write stats to log (with `info` level).

To enable metrics logging pass `--metrics_log` flag.

Your logs should contain something like this:

```sh
INFO 2018-03-06T14:16:27.872Z broadcast_msg_total=0 broadcast_streams_num=0 client_msg_total=0 clients_num=0 clients_uniq_num=0 context=metrics disconnect_queue_size=0 failed_auths_total=0 failed_broadcast_msg_total=0 failed_client_msg_total=0 goroutines_num=35 rpc_call_total=0 rpc_error_total=0
```

By default, metrics are logged every 15 seconds (you can change this behavior through `--metrics_rotate_interval` option).

### Custom loggers with mruby

<!-- TODO: add new API, remove "experimental" -->

> 👨‍🔬 This is an experimental API and could change in the future 👩‍🔬

AnyCable-Go allows you to write custom log formatters using an embedded [mruby](http://mruby.org) engine.

mruby is the lightweight implementation of the Ruby language. Hence it is possible to use Ruby to write metrics exporters.

First, you should download the version of `anycable-go` with mruby (it's not included by default): these binaries have `-mrb` suffix right after the version (i.e. `anycable-go-1.0.0-mrb-linux-amd64`).

**NOTE**: only MacOS and Linux are supported.

**NOTE**: when a server with mruby support is starting you should the following message:

```sh
$ anycable-go

INFO 2019-08-07T16:37:46.387Z context=main Starting AnyCable v0.6.2-13-gd421927 (with mruby 1.2.0 (2015-11-17)) (pid: 1362)
```

Secondly, write a Ruby script implementing a simple interface:

```ruby
# Module MUST be named MetricsFormatter
module MetricsFormatter
  # The only required method is .call.
  #
  # It accepts the metrics Hash and MUST return a string
  def self.call(data)
    data.to_json
  end
end
```

Finally, specify `--metrics_log_formatter` when running a server:

```sh
anycable-go --metrics_log_formatter path/to/custom_printer.rb
```

#### Example

This a [Librato](https://www.librato.com)-compatible printer:

```ruby
module MetricsFormatter
  def self.call(data)
    parts = []

    data.each do |key, value|
      parts << "sample##{key}=#{value}"
    end

    parts.join(" ")
  end
end
```

```sh
INFO 2018-04-27T14:11:59.701Z sample#clients_num=0 sample#clients_uniq_num=0 sample#goroutines_num=0
```
