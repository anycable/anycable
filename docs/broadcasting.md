# Broadcasting

Publishing messages from your application to connected clients (aka _broadcasting_) is an essential component of any real-time application.

AnyCable comes with multiple options on how to broadcast messages. We call them _broadcasters_. Currently, we support HTTP, Redis, and NATS-based broadcasters.

**NOTE:** The default broadcaster is Redis Pub/Sub for backward-compatibility reasons. This is going to change in v2.

## HTTP

> Enable via `--broadcast_adapter=http` (or `ANYCABLE_BROADCAST_ADAPTER=http`).

HTTP broadcaster has zero-dependencies and, thus, allows you to quickly start using AnyCable, and it's good enough to keep using it at scale.

By default, HTTP broadcaster accepts publications as POST requests to the `/_broadcast` path of your server\*. The request body MUST contain the publication payload (see below to learn about [the format](#publication-format)).

Here is a basic cURL example:

```bash
curl -X POST -H "Content-Type: application/json" -d '{"stream":"my_stream","data":"{\"text\":\"Hello, world!\"}"}' http://localhost:8090/_broadcast
```

\* If neither the broadcast key nor the application secret is specified, we configure HTTP broadcaster to use a different port by default (`:8090`) for security reasons. You can handle broadcast requests at the main AnyCable port by specifying it explicitly (via the `http_broadcast_port` option). If the broadcast key is specified or explicitly set to "none" or auto-generated from the application secret (see below), we run it on the main port. You will see the notice in the startup logs telling you how the HTTP broadcaster endpoint was configured:

```sh
2024-03-06 10:35:39.297 INF Accept broadcast requests at http://localhost:8090/_broadcast (no authorization) nodeid=uE3mZ7 context=broadcast provider=http

# OR
2024-03-06 10:35:39.297 INF Accept broadcast requests at http://localhost:8080/_broadcast (authorization required) nodeid=uE3mZ7 context=broadcast provider=http
```

### Securing HTTP endpoint

We automatically secure the HTTP broadcaster endpoint if the application broadcast key (`--broadcast_key`) is specified or inferred\* from the application secret (`--secret`) and the server is not running in the public mode (`--public`).

Every request MUST include an "Authorization" header with the `Bearer <broadcast-key>` value:

```sh
# Run AnyCable
$ anycable-go --broadcast_key=my-secret-key

2024-03-06 10:35:39.296 INF Starting AnyCable 1.5.0-a7aa9b4 (with mruby 1.2.0 (2015-11-17)) (pid: 57260, open file limit: 122880, gomaxprocs: 8) nodeid=uE3mZ7
...
2024-03-06 10:35:39.297 INF Accept broadcast requests at http://localhost:8080/_broadcast (authorization required) nodeid=uE3mZ7 context=broadcast provider=http

# Broadcast a message
$ curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer my-secret-key" -d '{"stream":"my_stream","data":"{\"text\":\"Hello, world!\"}"}' http://localhost:8080/_broadcast -w "%{http_code}"

201
```

\* When the broadcast key is missing but the application secret is present, we automatically generate a broadcast key using the following formula (in Ruby):

```ruby
broadcast_key = OpenSSL::HMAC.hexdigest("SHA256", "<APPLICATION SECRET>", "broadcast-cable")
```

When using official AnyCable server libraries, you don't need to calculate it yourself (they all use the same inference mechanism). But if you want to publish broadcasts using a custom implementation, you can generate a broadcast key for your secret key as follows:

```sh
echo -n 'broadcast-cable' | openssl dgst -sha256 -hmac '<your secret>' | awk '{print $2}'
```

## Redis Pub/Sub

> Enable via `--broadcast_adapter=redis` (or `ANYCABLE_BROADCAST_ADAPTER=redis`).

This broadcaster uses Redis [Pub/Sub](https://redis.io/topics/pubsub) feature under the hood, and, thus, publications are delivered to all subscribed AnyCable servers simultaneously.

All broadcast messages are published to a single channel (configured via the `--redis_channel`, defaults to `__anycable__`) as follows:

```sh
$ redis-cli PUBLISH __anycable__ '{"stream":"my_stream","data":"{\"text\":\"Hello, world!\"}"}'

(integer) 1
```

Note that since all AnyCable server receive each publication, we cannot use [broker](./broker.md) to provide stream history support when using Redis Pub/Sub.

See [configuration](./configuration.md#redis-configuration) for available Redis options.

## Redis X

> Enable via `--broadcast_adapter=redisx` (or `ANYCABLE_BROADCAST_ADAPTER=redisx`).

**IMPORTANT:** Redis v6.2+ is required.

Redis X broadcaster uses [Redis Streams][redis-streams] instead of Publish/Subscribe to _consume_ publications from your application. That gives us the following benefits:

- **Broker compatibility**. This broadcaster uses a [broker](/anycable-go/broker.md) to store messages in a cache and distribute them within a cluster. This is possible due to the usage of Redis Streams consumer groups.

- **Better delivery guarantees**. Even if there is no AnyCable server available at the broadcast time, the message will be stored in Redis and delivered to an AnyCable server once it is available. In combination with the [broker feature](./broker.md), you can achieve at-least-once delivery guarantees (compared to at-most-once provided by Redis Pub/Sub).

To broadcast a message, you publish it to a dedicated Redis stream (configured via the `--redis_channel` option, defaults to `__anycable__`) with the publication JSON provided as the `payload` field value:

```sh
$ redis-cli XADD __anycable__ "*" payload '{"stream":"my_stream","data":"{\"text\":\"Hello, world!\"}"}'

"1709754437079-0"
```

See [configuration](./configuration.md#redis-configuration) for available Redis options.

## NATS Pub/Sub

> Enable via `--broadcast_adapter=nats` (or `ANYCABLE_BROADCAST_ADAPTER=nats`).

NATS broadcaster uses [NATS publish/subscribe](https://docs.nats.io/nats-concepts/core-nats/pubsub) functionality and supports cluster features out-of-the-box. It works to Redis Pub/Sub: distribute publications to all subscribed AnyCable servers. Thus, it's incompatible with [broker](./broker.md) (stream history support), too.

To broadcast a message, you publish it to a NATS stream (configured via the `--nats_channel` option, defaults to `__anycable__`) as follows:

```sh
$ nats pub __anycable__ '{"stream":"my_stream","data":"{\"text\":\"Hello, world!\"}"}'

12:03:39 Published 60 bytes to "__anycable__"
```

NATS Pub/Sub is useful when you want to set up an AnyCable cluster using our [embedded NATS](./embedded_nats.md) feature, so you can avoid having additional infrastructure components.

See [configuration](./configuration.md#nats-configuration) for available NATS options.

## Publication format

AnyCable accepts broadcast messages encoded as JSON and having the following properties:

```js
{
  "stream": "<publication stream name>", // string
  "data": "<payload>", // string, usually a JSON-encoded object, but not necessarily
  "meta": "{}" // object, publication metadata, optional
}
```

It's also possible to publish multiple messages at once. For that, you just send them as an array of publications:

```js
[
  {
    "stream": "...",
    "data": "...",
  },
  {
    "stream": "...",
    "data": "..."
  }
]
```

The `meta` field MAY contain additional instructions for servers on how to deliver the publication. Currently, the following fields are supported:

- `exclude_socket`: you can specify a unique client identifier (returned by the server in the `welcome` message as `sid`) to remove this client from the list of recipients.

All other meta fields are ignored for now.

Here is a JSON Schema describing this format:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema",
  "definitions": {
    "publication": {
      "type": "object",
      "properties": {
      "stream": {
        "type": "string",
        "description": "Publication stream name"
      },
      "data": {
        "type": "string",
        "description": "Payload, usually a JSON-encoded object, but not necessarily"
      },
      "meta": {
        "type": "object",
        "description": "Publication metadata, optional",
        "properties": {
          "exclude_socket": {
            "type": "string",
            "description": "Unique client identifier to remove this client from the list of recipients"
          }
        },
        "additionalProperties": true
      }
    },
    "required": ["stream", "data"]
    }
  },
  "anyOf": [
    {
      "$ref": "#/definitions/publication"
    },
    {
      "type": "array",
      "items":{"$ref": "#/definitions/publication"}
    }
  ]
}
```

[redis-streams]: https://redis.io/docs/data-types/streams-tutorial/
