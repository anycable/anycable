# Broadcast Adapters

Broadcast adapter is used to proxy messaged published by your application to WebSocket server which in its turn broadcast messages to clients (see [architecture](../architecture.md)).

That is, when you call `ActionCable.server.broadcast`, AnyCable first pushes the message to WebSocket server via broadcast adapter, and the actual _broadcasting_ is happening within a WS server.

AnyCable ships with two broadcast adapters by default: HTTP and Redis (Redis is used by default).

## HTTP adapter

HTTP adapter has zero-dependencies and thus a good candidate for experimenting with AnyCable or even using in development/test environments.

**HTTP adapter is not meant for production**. It's not scalable (only supports a single WS server), less performant (due to HTTP overhead). For SSL connections it uses `SSL_VERIFY_NONE`.

To use HTTP adapter specify `broadcast_adapter` configuration parameter (`--broadcast-adapter=http` or `ANYCABLE_BROADCAST_ADAPTER=http` or set in the code/YML) and make sure your AnyCable WebSocket server supports it. An URL to broadcast to could be specified via `http_broadcast_url` parameter (defaults to `http://localhost:8080/_broadcast`, which corresponds to the [AnyCable-Go](../anycable-go/getting_started.md#configuration-parameters) default).

### Securing HTTP endpoint

Although the primary use-case for HTTP adapter is local development, you might want to use it in staging-like environments as well.
In this case, we recommend to protect the HTTP endpoint via a simple authorization via a header check.

You must configure both Ruby RPC server and a WebSocket server to use the same `http_broadcast_secret` (which will we passed via `Authorization: Bearer %secret%`).

## Redis adapter

It's a default adapter to AnyCable. It uses Redis [Pub/Sub](https://redis.io/topics/pubsub) feature under the hood.

**NOTE:** To use Redis adapter, you must ensure that it is present in your Gemfile; AnyCable gem doesn't have `redis` as a dependency.

See [configuration](./configuration.md) for available Redis options.

### Redis Sentinel support

AnyCable could be used with Redis Sentinel out-of-the-box. For that, you should configure it the following way:

- `redis_url` must contain a master name (e.g., `ANYCABLE_REDIS_URL=redis://mymaster`)
- `redis_sentinels` must contain a comma separated list of sentinel hosts (e.g., `ANYCABLE_REDIS_SENTINELS=my.redis.sentinel.first:26380,my.redis.sentinel.second:26380`).

If your sentinels are protected with passwords, use the following format: `:password1@my.redis.sentinel.first:26380,:password2@my.redis.sentinel.second:26380`.

> See the [demo](https://github.com/anycable/anycable_rails_demo/pull/8) of using Redis with Sentinels in a local Docker dev environment.

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
