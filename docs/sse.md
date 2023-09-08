# Server-sent events

In addition to WebSockets and [long polling](./long_polling.md)), AnyCable also allows you to use [Server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events) (SSE) as a transport for receiving live updates.

SSE is supported by all modern browsers (see [caniuse](https://caniuse.com/eventsource)) and is a good alternative to WebSockets if you don't need to send messages from the client to the server and don't want to deal with Action Cable or AnyCable SDKs: you can use native browsers `EventSource` API to establish a reliable connection or set up HTTP streaming manually using your tool of choice (e.g., `fetch` in a browser, `curl` in a terminal, etc.).

## Configuration

You must opt-in to enable SSE support in AnyCable. To do so, you must provide the `--sse` or set the `ANYCABLE_SSE` environment variable to `true`:

```sh
$ anycable-go --sse

INFO 2023-09-06T22:52:04.229Z context=main Starting GoBenchCable 1.4.4 (with mruby 1.2.0 (2015-11-17)) (pid: 39193, open file limit: 122880, gomaxprocs: 8)
...
INFO 2023-09-06T22:52:04.229Z context=main Handle WebSocket connections at http://localhost:8080/cable
INFO 2023-09-06T22:52:04.229Z context=main Handle SSE requests at http://localhost:8080/events
...
```

The default path for SSE connections is `/events`, but you can configure it via the `--sse_path` configuration option.

## Usage with EventSource

The easiest way to use SSE is to use the native `EventSource` API. For that, you MUST provide a URL to the SSE endpoint and pass the channel information in query parameters (EventSource only supports GET requests, so we cannot use the body). For example:

```js
const source = new EventSource("http://localhost:8080/events?channel=ChatChannel");

// Setup an event listener to handle incoming messages
source.addEventListener("message", (e) => {
  // e.data contains the message payload as a JSON string, so we need to parse it
  console.log(JSON.parse(e.data));
});
```

The snippet above will establish a connection to the SSE endpoint and subscribe to the `ChatChannel` channel.

If you need to subscribe to a channel with parameters, you MUST provide the fully qualified channel identifier via the `identifier` query parameter:

```js
const identifier = JSON.stringify({
  channel: "BenchmarkChannel",
  room_id: 42,
});

const source = new EventSource(
  `http://localhost:8080/events?identifier=${encodeURIComponent(identifier)}`
);

// ...
```

**IMPORTANT**: You MUST specify either `channel` or `identifier` query parameters. If you don't, the connection will be rejected.

### Reliability

EventSource is a reliable transport, which means that it will automatically reconnect if the connection is lost.

EventSource also keeps track of received messages and sends the last consumed ID on reconnection. To leverage this feature, you MUST enable AnyCable [reliable streams](./reliable_streams.md) functionality. No additional client-side configuration is required.

**IMPORTANT**: EventSource is assumed to be used with a single stream of data. If you subscribe a client to multiple Action Cable streams (e.g., multiple `stream_from` calls), the last consumed ID will be sent only for the last observed stream.

### Unauthorized connections or rejected subscriptions

If the connection is unauthorized or the subscription is rejected, the server will respond with a `401` status code and close the connection. EventSource will automatically reconnect after a short delay. Please, make sure you handle `error` events and close the connection if you don't want to reconnect.

## Usage with other HTTP clients

You can also use any other HTTP client to establish a connection to the SSE endpoint. For example, you can use `curl`:

```sh
$ curl -N "http://localhost:8080/events?channel=ChatChannel"

event: welcome
data: {"type":"welcome"}

event: confirm_subscription
data: {"type":"confirm_subscription","identifier":"{\"channel\":\"ChatChannel\"}"}

event: ping
data: {"type":"ping","message":1694041735}

data: {"message":"hello"}
...
```

AnyCable also supports setting up a streaming HTTP connection via POST requests. In this case, you can provide a list of client-server commands in the request body using the JSONL (JSON lines) format.

<!-- TODO: fetch example -->

Note that you must process different event types yourself. See below for the format.

## Action Cable over SSE format

The server-client communication format is designed as follows:

- The `data` field contains the message payload. **IMPORTANT**: for clients connecting via a GET request, the payload only contains the `message` part of the original Action Cable payload; clients connecting via POST requests receive the full payload (e.g., `{"identifier":, "message": {"foo":1}}`).
- The optional `event` field contains the message type (if any); for example, `welcome`, `confirm_subscription`, `ping`
- The optional `id` field contains the message ID if reliable streaming is enabled. The message ID has a form or `<offset>/<stream_id>/<epoch>` (see [Extended Action Cable protocol](/misc/action_cable_protocol.md#action-cable-extended-protocol))
- The optional `retry` field contains the reconnection interval in milliseconds. We only set this field for `disconnect` messages with `reconnect: false` (it's set to a reasonably high number to prevent automatic reconnection attempts by EventSource).

Here is an example of a stream of messages from the server:

```txt
event: welcome
data: {"type":"welcome"}


event: confirm_subscription
data: {"type":"confirm_subscription","identifier":"{\"channel\":\"ChatChannel\"}"}


event: ping
data: {"type":"ping","message":1694041735}


<!-- GET connection (e.g., EventSource) -->
data: {"message":"hello"}
id: 1/chat_42/y2023

data: {"message":"good-bye"}
id: 2/chat_42/y2023


<!-- POST connection  -->
data: {"identifier":"{\"channel\":\"ChatChannel\"}","message":{"message":"hello"}}
id: 1/chat_42/y2023


data: {"identifier":"{\"channel\":\"ChatChannel\"}","message":{"message":"good-bye"}}
id: 2/chat_42/y2023


event: ping
data: {"type":"ping","message":1694044435}


event: disconnect
data: {"type":"disconnect","reason":"remote","reconnect":false}
retry: 31536000000
```
