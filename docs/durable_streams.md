# Durable Streams

AnyCable supports the [Durable Streams](https://github.com/durable-streams/durable-streams) specification, enabling HTTP-based clients to consume real-time data streams using catch-up reads, long-polling, and Server-Sent Events (SSE).

> **Note**: AnyCable implements only the **read** portion of the Durable Streams specification. Write operations are not supported.

## Overview

Durable Streams provides a standardized HTTP API for consuming real-time data with built-in support for:

- **Catch-up reads**: Fetch historical messages from a stream
- **Long-polling**: Wait for new messages with automatic timeout
- **SSE streaming**: Continuous real-time updates via Server-Sent Events

This is particularly useful for clients that cannot use WebSockets or prefer HTTP-based communication. Or for platforms/environments, where you can't use AnyCable or Action Cable client SDKs.

## Configuration

Enable Durable Streams support by setting the `--ds` flag or `ANYCABLE_DS=true` environment variable:

```sh
$ anycable-go --ds

INFO ... Handle Durable Streams requests at http://localhost:8080/ds
```

### Configuration options

| Option | Env variable | Default | Description |
|--------|--------------|---------|-------------|
| `--ds` | `ANYCABLE_DS` | `false` | Enable Durable Streams support |
| `--ds_path` | `ANYCABLE_DS_PATH` | `/ds` | URL path for DS requests |
| `--ds_skip_auth` | `ANYCABLE_DS_SKIP_AUTH` | `false` | Skip client authentication (only authorize streams) |
| `--ds_poll_interval` | `ANYCABLE_DS_POLL_INTERVAL` | `10` | Long-poll timeout in seconds |
| `--ds_sse_ttl` | `ANYCABLE_DS_SSE_TTL` | `60` | Maximum SSE connection lifetime in seconds |

## Client usage

Use the official [@durable-streams/client](https://www.npmjs.com/package/@durable-streams/client) SDK to consume streams:

```js
import { stream } from "@durable-streams/client";

const baseUrl = "http://localhost:8080";
const streamName = "chat/room-42";

// Catch-up read (fetch existing messages)
const res = await stream({
  url: `${baseUrl}/ds/${streamName}`,
  offset: "-1", // Start from beginning
});

const messages = await res.json();
console.log("Messages:", messages);
console.log("Next offset:", res.offset);
```

### Reading modes

#### Catch-up mode

Fetch historical messages without waiting for new ones:

```js
const res = await stream({
  url: `${baseUrl}/ds/${streamName}`,
  offset: "-1", // -1 means start from beginning
});

const messages = await res.json();
// Use res.offset for subsequent requests
```

#### Long-poll mode

Wait for new messages with automatic timeout:

```js
const res = await stream({
  url: `${baseUrl}/ds/${streamName}`,
  offset: lastOffset, // Required for live modes
  live: "long-poll",
});

const messages = await res.json();
```

#### SSE mode

Continuous streaming with automatic reconnection support:

```js
const res = await stream({
  url: `${baseUrl}/ds/${streamName}`,
  offset: "-1",
  live: "sse",
  json: true,
});

res.subscribeJson(async (batch) => {
  for (const item of batch.items) {
    console.log("Received:", item);
  }
});
```

## Authentication and authorization

AnyCable DS supports two layers of security: **client authentication** and **stream authorization**.

### Client authentication

By default, DS requests go through the same authentication flow as WebSocket connections. You can use [JWT authentication](./jwt_identification.md) for stateless authentication:

```js
const res = await stream({
  url: `${baseUrl}/ds/${streamName}`,
  offset: "-1",
  headers: {
    "X-JID": jwtToken, // Or use the configured header name
  },
});
```

To skip client authentication and only perform stream authorization, set `--ds_skip_auth=true`.

### Stream authorization

Stream access is controlled using [signed streams](./signed_streams.md). Provide a signed stream token via query parameter or header:

```js
// Via query parameter
const res = await stream({
  url: `${baseUrl}/ds/${streamName}?signed=${signedToken}`,
  offset: "-1",
});

// Via header
const res = await stream({
  url: `${baseUrl}/ds/${streamName}`,
  offset: "-1",
  headers: {
    "X-Signed": signedToken,
  },
});
```

Generate signed tokens using the same mechanism as for WebSocket signed streams:

```ruby
# Ruby/Rails
signed_token = AnyCable::Streams.signed("chat/room-42")
```

```js
// Node.js (using @anycable/serverless-js)
import { createHmac } from 'crypto';

const encoded = Buffer.from(JSON.stringify(streamName)).toString('base64');
const digest = createHmac('sha256', streamsSecret).update(encoded).digest('hex');
const signedToken = `${encoded}--${digest}`;
```

If public streams are enabled (`--public_streams`), unsigned stream names are also accepted.

## Requirements

Durable Streams requires a [broker](./broker.md) to be configured for message history:

```sh
$ anycable-go --ds --broker=memory
```

See [reliable streams](./reliable_streams.md) for more information on broker configuration and cache settings.

## Limitations

- Only `application/json` content type is supported
- Write operations (appends, stream creation) are not implemented; use general AnyCable [broadcasting](./broadcasting.md) capabilities.
- Offsets are opaque tokens specific to AnyCable; clients should not parse them
