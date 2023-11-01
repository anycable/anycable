# Reliable streams and resumable sessions

Since v1.4, AnyCable allows you to enhance the consistency of your real-time data and go from **at-most-once** to **at-least-once** and even **exactly-once delivery**.

> 🎥 Learn more about the consistency pitfalls of Action Cable from [The pitfalls of realtime-ification](https://noti.st/palkan/MeBUVe/the-pitfalls-of-realtime-ification) talk (RailsConf 2022).

## Overview

The next-level delivery guarantees are achieved by introducing **reliable streams**. AnyCable keeps a *hot cache* \* of the messages sent to the streams and allows clients to request the missed messages on re-connection.

In addition to reliable streams, AnyCable v1.4 also introduces **resumable sessions**. This feature allows clients to restore their state on re-connection and avoid re-authentication and re-subscription to channels.

\* The "hot cache" here means that the cache is short-lived and is not intended to be used as a long-term storage. The primary purpose of this cache is to improve the reliability of the stream delivery for clients with unstable network connections.

## Quick start

The easiest way to try the streams history feature is to use the `broker` preset for AnyCable-Go (named after the underlying component, [Broker](./broker.md)):

```sh
$ anycable-go --presets=broker

INFO 2023-04-14T00:31:55.548Z context=main Starting AnyCable 1.4.0-d8939df (with mruby 1.2.0 (2015-11-17)) (pid: 87410, open file limit: 122880, gomaxprocs: 8)
INFO 2023-04-14T00:31:55.548Z context=main Using in-memory broker (epoch: vRXl, history limit: 100, history ttl: 300s, sessions ttl: 300s)
INFO 2023-04-18T20:46:00.693Z context=pubsub Starting Redis pub/sub: localhost:6379
INFO 2023-04-19T16:22:55.776Z context=pubsub provider=http Accept broadcast requests at http://localhost:8090/_broadcast
...
```

Now, at the Ruby/Rails side, switch to the `http` or `redisx` broadcasting adapter (if you use Redis). For example, in `config/anycable.yml`:

```yaml
default: &default
  # ...
  broadcast_adapter: http
```

Finally, at the client-side, you MUST use the [AnyCable JS client](https://github.com/anycable/anycable-client) and configure it to use the `actioncable-v1-ext-json` protocol:

```js
import { createCable } from '@anycable/web'
// or for non-web projects
// import { createCable } from '@anycable/core'

export default createCable({protocol: 'actioncable-v1-ext-json'})
```

That's it! Now your clients will automatically catch-up with the missed messages and restore their state on re-connection.

## Manual configuration

The `broker` preset is good for a quick start. Let's see how to configure the broker and other components manually.

First, you need to provide the `--broker` option with a broker adapter name:

```sh
$ anycable-go --broker=memory

INFO 2023-04-14T00:31:55.548Z context=main Starting AnyCable 1.4.0-d8939df (with mruby 1.2.0 (2015-11-17)) (pid: 87410, open file limit: 122880, gomaxprocs: 8)
INFO 2023-04-14T00:31:55.548Z context=main Using in-memory broker (epoch: vRXl, history limit: 100, history ttl: 300s, sessions ttl: 300s)
...
```

Then, you MUST configure a compatible broadcasting adapter (currently, `http` and `redisx` are available). For example, when using Redis:

```sh
$ anycable-go --broker=memory --broadcast_adapter=redisx

...
INFO 2023-07-04T02:00:24.386Z consumer=s2IbkM context=broadcast id=s2IbkM provider=redisx stream=__anycable__ Starting Redis broadcaster at localhost:6379
...
```

See [Broadcast adapters](/ruby/broadcast_adapters.md) for more information.

Finally, to re-transmit _registered_ messages within a cluster, you MUST also configure a pub/sub adapter (via the `--pubsub` option). The command will look as follows:

```sh
$ anycable-go --broker=memory --broadcast_adapter=redisx --pubsub=redis

INFO 2023-07-04T02:02:10.548Z context=main Starting AnyCable 1.4.0-d8939df (with mruby 1.2.0 (2015-11-17)) (pid: 87410, open file limit: 122880, gomaxprocs: 8)
INFO 2023-07-04T02:02:10.548Z context=main Using in-memory broker (epoch: vRXl, history limit: 100, history ttl: 300s, sessions ttl: 300s)
INFO 2023-07-04T02:02:10.586Z consumer=s2IbkM context=broadcast id=s2IbkM provider=redisx stream=__anycable__ Starting Redis broadcaster at localhost:6379
INFO 2023-07-04T02:02:10.710Z context=pubsub Starting Redis pub/sub: localhost:6379
...
```

See [Pub/Sub documentation](./pubsub.md) for available options.

### Cache settings

There are several configuration options to control how to store messages and sessions:

- `--history_limit`: Max number of messages to keep in the stream's history. Default: `100`.
- `--history_ttl`: Max time to keep messages in the stream's history. Default: `300s`.
- `--sessions_ttl`: Max time to keep sessions in the cache. Default: `300s`.

Currently, the configuration is global. We plan to add support for granular (per-stream) settings in the following releases.

## Resumed sessions vs. disconnect callbacks

AnyCable WebSocket server notifies a main application about the client disconnection via the `Disconnect` RPC call (which translates into `Connection#disconnect` and `Channel#unsubscribed` calls in Rails). Currently, when the client's session is restored, no callbacks are invoked in the main application. Keep this limitation in mind when designing your business logic (i.e., if you rely on connect/disconnect callbacks, you should consider disabling sessions cache by setting `sessions_ttl` to 0).

**NOTE:** We consider introducing a new RPC method, `Restore`, along with the corresponding Ruby-side callbacks (`Connection#restored` and `Channel#resubscribed`) to handle this situation. Feel free to join [the discussion](https://github.com/orgs/anycable/discussions/209) and share your thoughts!

## Cache backends

### Memory

The default broker adapter. It stores all data in memory. It can be used **only for single node installations**.

**IMPORTANT**: Since the data is stored in memory, it's getting lost during restarts.

**NOTE:** Storing data in memory may result into the increased RAM usage of an AnyCable-Go process.

### NATS

_🧪 Experimental_

This adapter uses [NATS JetStream](https://nats.io/) as a shared distributed storage for sessions and streams cache and also keeps a local snapshot in memory (using the in-memory adapter described above).

It can be used with both external NATS and [embedded NATS](./embedded_nats.md):

```sh
$ anycable-go --broker=nats --nats_servers=nats://localhost:4222

  INFO 2023-10-28T00:57:53.937Z context=main Starting AnyCable 1.4.6-c31c153 (with mruby 1.2.0 (2015-11-17)) (pid: 29874, open file limit: 122880, gomaxprocs: 8)
  INFO 2023-10-28T00:57:53.937Z context=main Starting NATS broker: nats://localhost:4222 (history limit: 100, history ttl: 300s, sessions ttl: 300s)
  ...
```

Or with embedded NATS:

```sh
$ anycable-go --embed_nats --broker=nats

  INFO 2023-10-28T00:59:01.177Z context=main Starting AnyCable 1.4.6-c31c153 (with mruby 1.2.0 (2015-11-17)) (pid: 30693, open file limit: 122880, gomaxprocs: 8)
  INFO 2023-10-28T00:59:01.177Z context=main Starting NATS broker: nats://127.0.0.1:4222 (history limit: 100, history ttl: 300s, sessions ttl: 300s)
  INFO 2023-10-28T00:59:01.205Z context=main Embedded NATS server started: nats://127.0.0.1:4222
  ...
```

### Redis

<p class="pro-badge-header"></p>

AnyCable-Go Pro comes with a Redis-based broker adapter. It stores all data in Redis and, thus, can be used in multi-node installations.

To use Redis broker, you need to provide the `--broker` option with the `redis` adapter name:

```sh
$ anycable-go --broker=redis

 INFO 2023-07-08T00:46:55.491Z context=main Starting AnyCable 1.4.0-pro-eed05bc (with mruby 1.2.0 (2015-11-17)) (pid: 78585, open file limit: 122880, gomaxprocs: 8, netpoll: true)
 INFO 2023-07-08T00:46:55.492Z context=main Using Redis broker at localhost:6379 (history limit: 100, history ttl: 300s, sessions ttl: 300s)
 ...
```

When you use the `broker` preset with AnyCable-Go, it automatically configures the Redis broker (if Redis credentials are configured).

#### Streams history expiration

We use [Redis Streams](https://redis.io/docs/data-types/streams/) to store messages history. Redis doesn't support expiring individual messages in a stream, so we expire the whole stream instead. In other words, the `--history_ttl` option controls the expiration of the whole stream.

## Further reading

For in-depth information about the feature and its internals, see the following articles:

- [Broker](./broker.md)
- [Pub/Sub](./pubsub.md)
