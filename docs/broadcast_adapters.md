# Broadcasting

AnyCable supports multiple ways of publishing messages from your backend to connected clients: HTTP API, Redis and [NATS][]-backed.

AnyCable Ruby provides a universal API to publish broadcast messages from your Ruby/Rails applications independently of which underlying technology you would like to use. All you need is to pick and configure an adapter.

Learn more about different broadcasting options and when to prefer one over another in the [AnyCable broadcasting documentation](../anycable-go/broadcasting.md).

## Configuration

By default, AnyCable uses Redis Pub/Sub adapter (`redis`). Use the `broadcast_adapter` (`ANYCABLE_BROADCAST_ADAPTER`) configuration parameter to use another one.

### HTTP

> Enable via `broadcast_adapter: http` in `anycable.yml` or `ANYCABLE_BROADCAST_ADAPTER=http`.

The following configuration options are available:

- **http_broadcast_url** (`ANYCABLE_HTTP_BROADCAST_URL`)

  Specify AnyCable HTTP broadcasting endpoint. Defaults to `http://localhost:8090/_broadcast`.

If your HTTP broadcasting endpoint is secured, use the `broadcast_key` option to provide the key or the application secret (`secret`) to auto-generate it (the configuration MUST match your AnyCable server configuration).

### Redis Pub/Sub

> Enable via `broadcast_adapter: redis` in `anycable.yml` or `ANYCABLE_BROADCAST_ADAPTER=redis`.

**NOTE:** To use Redis adapters, you MUST add the `redis` gem to your Gemfile yourself.

The following configuration options are available:

- **redis_url** (`REDIS_URL`, `ANYCABLE_REDIS_URL`)

  Redis connection URL (MAY include auth credentials) (default: `"redis://localhost:6379"`).

- **redis_channel** (`ANYCABLE_REDIS_CHANNEL`)

  Redis channel used for broadcasting (default: `"__anycable__"`).

- **redis_tls_verify** (`ANYCABLE_REDIS_TLS_VERIFY`)

  Whether to validate Redis server TLS certificate if `rediss://` protocol is used (default: `false`).

- **redis_tls_client_cert_path** (`ANYCABLE_REDIS_TLS_CLIENT_CERT_PATH`, `--redis-tls-client_cert-path`)

  Path to the file with a client TLS certificate in PEM format if the Redis server requires client authentication.

- **redis_tls_client_key_path** (`ANYCABLE_REDIS_TLS_CLIENT_KEY_PATH`)

  Path to the file with a private key for the client TLS certificate if the Redis server requires client authentication.

**NOTE:** Redis broadcast adapter uses a single connection to Redis.

#### Redis Sentinel support

AnyCable could be used with Redis Sentinel out-of-the-box. For that, you should configure it the following way:

- `redis_url` MUST contain a master name (e.g., `ANYCABLE_REDIS_URL=redis://mymaster`)
- `redis_sentinels` MUST contain a comma separated list of sentinel hosts (e.g., `ANYCABLE_REDIS_SENTINELS=my.redis.sentinel.first:26380,my.redis.sentinel.second:26380`).

If your sentinels are protected with passwords, use the following format: `:password1@my.redis.sentinel.first:26380,:password2@my.redis.sentinel.second:26380`.

> See the [demo](https://github.com/anycable/anycable_rails_demo/pull/8) of using Redis with Sentinels in a local Docker dev environment.

### Redis Streams

> Enable via `broadcast_adapter: redisx` in `anycable.yml` or `ANYCABLE_BROADCAST_ADAPTER=redisx`.

Redis Streams broadcaster shares configuration settings with Redis Pub/Sub (see above).
The `redis_channel` value used as the name of the Redis Stream to publish broadcasts to.

### NATS Pub/Sub

> Enable via `broadcast_adapter: nats` in `anycable.yml` or `ANYCABLE_BROADCAST_ADAPTER=nats`.

**NOTE:** Make sure you added [`nats-pure` gem][nats-pure] to your Gemfile.

The following configuration options are available:

- **nats_servers** (`ANYCABLE_NATS_SERVERS`, `--nats-servers`)

  A comma-separated list of NATS server addresses (default: `"nats://localhost:4222"`).

- **nats_channel** (`ANYCABLE_NATS_CHANNEL`, `--redis-channel`)

  NATS pus/sub channel for broadcasting (default: `"__anycable__"`).

With [embedded NATS](../anycable-go/embedded_nats.md) feature of AnyCable, you can minimize the number of required components to deploy an AnyCable-backed application.

## Broadcasting API

To publish a message to a stream via AnyCable, you can use the following API:

```ruby
AnyCable.broadcast("my_stream", {text: "hoi"})

# or directly via the singleton broadcast adapter instance
AnyCable.broadcast_adapter.broadcast("my_stream", {text: "hoi"})
```

### Batching

AnyCable-Go v1.4.5+ supports publishing broadcast messages in batches. This is especially useful if you want to guarantee the order of delivered messages to clients (to be the same as the broadcasts order). To batch-broadcast messages, wrap your code with the `.batching` method of the broadcast adapter:

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

### Broadcast options

AnyCable v1.4.5+ supports additional broadcast options. You can pass them as the third argument to the `AnyCable.broadcast` method:

```ruby
AnyCable.broadcast("my_stream", {text: "hoi"}, {exclude_socket: "some-socket-id"})
```

The following options are supported:

- `exclude_socket`: pass an AnyCable socket ID to exclude it from the broadcast recipients list. Useful if you want to broadcast to all clients except the one that initiated the broadcast.

[NATS]: https://nats.io
[nats-pure]: https://github.com/nats-io/nats-pure.rb
