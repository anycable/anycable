# Broadcast Adapters

Broadcast adapter is used to handle messages published by your application to WebSocket server which in its turn delivers the messages to connected clients (see [architecture](../architecture.md)).

That is, when you call `ActionCable.server.broadcast`, AnyCable first pushes the message to WebSocket server via a broadcast adapter, and the actual _broadcasting_ is happening within a WS server (or servers).

AnyCable ships with three broadcast adapters by default: Redis (default), [NATS][], and HTTP.

**NOTE:** The broadcast adapters fall into two categories: legacy fan-out (distributed) and _singular_.

## Broadcasting API

To publish a message to a stream via AnyCable, you can use the following API:

```ruby
AnyCable.broadcast("my_stream", {text: "hoi"})

# or directly via the singleton broadcast adapter instance
AnyCable.broadcast_adapter.broadcast("my_stream", {text: "hoi"})
```

### Batching

Since v1.4.5, AnyCable-Go supports publishing broadcast messages in batches. This is especially useful if you want to guarantee the order of delivered messages to clients (to be the same as the broadcasts order). To batch-broadcast messages, wrap your code with the `.batching` method of the broadcast adapter:

```ruby
AnyCable.broadcast_adapter.batching do
  AnyCable.broadcast("my_stream", {text: "hoi"})
  AnyCable.broadcast("my_stream", {text: "wereld"})
end #=> the actual publishing happens as we exit the block
```

The `.batching` method supports nesting, if you need to broadcast some messages immediately:

```ruby
AnyCable.broadcast_adapter.batching do
  AnyCable.broadcast("my_stream", {text: "hoi"}) # added to the current batch

  AnyCable.broadcast_adapter.batching(false) do
    AnyCable.broadcast("another_stream", {text: "some other story"}) #=> publish immediately

    AnyCable.broadcast_adapter.batching do
      AnyCable.broadcast("my_stream", {text: "wereld"}) # added to the current batch
    end
  end
end #=> the current batch is published
```

## HTTP adapter

HTTP adapter has zero-dependencies and, thus, allows you to quickly start using AnyCable.

Since v1.4, HTTP adapter can also be considered for production (thanks to the new [pub/sub component](/anycable-go/pubsub.md) in AnyCable-Go). Moreover, it can be used with the new [broker feature](/anycable-go/broker.md) of AnyCable-Go.

To use HTTP adapter specify `broadcast_adapter` configuration parameter (`--broadcast-adapter=http` or `ANYCABLE_BROADCAST_ADAPTER=http` or set in the code/YML) and make sure your AnyCable WebSocket server supports it. An URL to broadcast to could be specified via `http_broadcast_url` parameter (defaults to `http://localhost:8090/_broadcast`, which corresponds to the [AnyCable-Go](../anycable-go/getting_started.md#configuration-parameters) default).

**NOTE:** For SSL connections, we use the `SSL_VERIFY_NONE` mode.

Example cURL command to publish a message:

```bash
curl -X POST -H "Content-Type: application/json" -d '{"stream":"my_stream","data":"{\"text\":\"Hello, world!\"}"}' http://localhost:8090/_broadcast
```

### Securing HTTP endpoint

If your broadcasting HTTP endpoint is open to public, we recommend to protect it via a simple authorization via a header check.

You must configure both Ruby RPC server and a WebSocket server to use the same `http_broadcast_secret` (which will we passed via `Authorization: Bearer %secret%`).

## Redis X

Redis X adapter uses [Redis Streams][redis-streams] instead of Publish/Subscribe to deliver broadcasting messages from your application to WebSocket servers. That gives us the following benefits:

- **Better delivery guarantees**. Even if there is no WebSocket server available at the broadcast time, the message will be stored in Redis and delivered to the server once it is available. In combination with the [broker feature](/anycable-go/broker.md), you can achieve at-least-once delivery guarantees (compared to at-most-once provided by Redis pub/sub).

- **Broker compatibility**. Using a [broker](/anycable-go/broker.md) (or **streams history**) requires handling each broadcasted message by a single node in the cluster (so it can be _registered_ in a cache). With Redis X adapter, we achieve this by using consumer groups for the Redis stream.

Configuration options are the same as for the Redis adapter. The `redis_channel` option is treated as a stream name.

**IMPORTANT:** Redis v6.2+ is required.

See [configuration](./configuration.md) for available Redis options.

## Redis adapter (legacy)

It's a default adapter for AnyCable. It uses Redis [Pub/Sub](https://redis.io/topics/pubsub) feature under the hood. Thus, all the messages delivered to all WebSocket servers at once.

**NOTE:** To use Redis adapter, you must ensure that it is present in your Gemfile; AnyCable gem doesn't have `redis` as a dependency.

See [configuration](./configuration.md) for available Redis options.

### Redis Sentinel support

AnyCable could be used with Redis Sentinel out-of-the-box. For that, you should configure it the following way:

- `redis_url` must contain a master name (e.g., `ANYCABLE_REDIS_URL=redis://mymaster`)
- `redis_sentinels` must contain a comma separated list of sentinel hosts (e.g., `ANYCABLE_REDIS_SENTINELS=my.redis.sentinel.first:26380,my.redis.sentinel.second:26380`).

If your sentinels are protected with passwords, use the following format: `:password1@my.redis.sentinel.first:26380,:password2@my.redis.sentinel.second:26380`.

> See the [demo](https://github.com/anycable/anycable_rails_demo/pull/8) of using Redis with Sentinels in a local Docker dev environment.

## NATS adapter (legacy)

**NOTE:** Make sure you added [`nats-pure` gem][nats-pure] to your Gemfile.

NATS adapter uses [NATS publish/subscribe](https://docs.nats.io/nats-concepts/core-nats/pubsub) functionality and supports cluster features out-of-the-box.

> With [embedded NATS](../anycable-go/embedded_nats.md) feature of AnyCable-Go, you can minimize the number of required components to deploy an AnyCable-backed application.

See [configuration](./configuration.md) for available NATS options.

## Custom adapters

AnyCable allows you to use custom broadcasting adapters:

```ruby
# Specify by name (tries to load `AnyCable::BroadcastAdapters::MyAdapter` from
# "anycable/broadcast_adapters/my_adapter")
AnyCable.broadcast_adapter = :my_adapter, {option: "value"}
# or provide an instance (should respond_to #broadcast)
AnyCable.broadcast_adapter = MyAdapter.new
```

Want to have a different adapter out-of-the-box? Join [the discussion](https://github.com/anycable/anycable/issues/2).

[NATS]: https://nats.io
[nats-pure]: https://github.com/nats-io/nats-pure.rb
[redis-streams]: https://redis.io/docs/data-types/streams-tutorial/
