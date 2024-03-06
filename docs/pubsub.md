# Pub/Sub for node-node communication

When running multiple instances of AnyCable-Go, you have two options to deliver broadcast messages to all nodes (and, thus, clients connected to each node). The first, legacy option is to use a fan-out, or distributed broadcasting adapter (Redis or NATS), i.e., deliver messages to all nodes simultaneously and independently. The second option is to publish a message to a single node (picked randomly), and then let AnyCable-Go to re-transmit it to other nodes from the cluster. The latter is **required** for multi-node setups with a [broker](./broker.md).

Although, we do not plan to sunset legacy, distributed adapters in the nearest future, we recommend switching to a new broadcaster+pubsub (_B+P/S_) architecture for the following reasons:

- Broker (aka _streams history_) requires the new architecture.
- The new architecture scales better by avoiding sending **all** messages to **all** nodes; only the nodes _interested_ in a particular stream (i.e., having active subscribers) receive broadcast messages via pub/sub.

**NOTE:** The new architecture will be the default one since v1.5.

## Usage

By default, pub/sub is disabled (since the default broadcast adapter is legacy, fan-out Redis). To enable the pub/sub layer, you must provide the name of the provider via the `--pubsub` option.

You also need to enable a compatible broadcasting adapter. See [broadcasting](./broadcasting.md).

**NOTE**: It's safe to enable `--pubsub` even if you're still using legacy broadcasting adapters (they do not pass messages through the pub/sub layer).

## Supported adapters

### Redis

The Redis pub/sub adapter uses the Publish/Subscribe Redis feature to re-transmit messages within a cluster. To enable it, set the value of the`pubsub` parameter to `redis`:

```sh
$ anycable-go --pubsub=redis
# or
$ ANYCABLE_PUBSUB=redis anycable-go

INFO 2023-04-18T20:46:00.692Z context=main Starting AnyCable 1.4.0-36a43e5 (with mruby 1.2.0 (2015-11-17)) (pid: 16574, open file limit: 122880, gomaxprocs: 8)
INFO 2023-04-18T20:46:00.693Z context=pubsub Starting Redis pub/sub: localhost:6379
...
```

See [configuration](./configuration.md) for available Redis configuration settings.

### NATS

```sh
$ anycable-go --pubsub=nats
# or
$ ANYCABLE_PUBSUB=nats anycable-go

INFO 2023-04-18T20:28:38.410Z context=main Starting AnyCable 1.4.0-36a43e5 (with mruby 1.2.0 (2015-11-17)) (pid: 9125, open file limit: 122880, gomaxprocs: 8)
INFO 2023-04-18T20:28:38.411Z context=pubsub Starting NATS pub/sub: nats://127.0.0.1:4222
...
```

You can use it with the [embedded NATS](./embedded_nats.md), too:

```sh
$ anycable-go --embed_nats --pubsub=nats

INFO 2023-04-18T20:30:58.724Z context=main Starting AnyCable 1.4.0-36a43e5 (with mruby 1.2.0 (2015-11-17)) (pid: 9615, open file limit: 122880, gomaxprocs: 8)
INFO 2023-04-18T20:30:58.753Z context=main Embedded NATS server started: nats://127.0.0.1:4222
INFO 2023-04-18T20:30:58.755Z context=pubsub Starting NATS pub/sub: nats://127.0.0.1:4222
```

See [configuration](./configuration.md) for available NATS configuration settings.
