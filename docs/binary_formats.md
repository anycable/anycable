# Binary messaging formats <img class='pro-badge' src='https://docs.anycable.io/assets/pro.svg' alt='pro' />

AnyCable Pro allows you to use Msgpack or Protobufs instead of JSON to serialize incoming and outgoing data. Using binary formats bring the following benefits: faster (de)serialization and less data passing through network (see comparisons below).

## Msgpack

### Usage

In order to initiate Msgpack-encoded connection, a client MUST use `"actioncable-v1-msgpack"` subprotocol during the connection (instead of the `"actioncable-v1-json"`).

A client MUST encode outgoing and incoming messages using Msgpack.

### Action Cable JavaScript client patch

Here is how you can patch the built-in Action Cable JavaScript client library to support Msgpack:

```js
import { createConsumer, logger, adapters, INTERNAL } from "@rails/actioncable";
// Make sure you added msgpack library to your frontend bundle:
//
//    yarn add @ygoe/msgpack
//
import msgpack from "@ygoe/msgpack";

let consumer;

// This is an application specific function to create an Action Cable consumer.
// Use it everywhere you need to connect to Action Cable.
export const createCable = () => {
  if (!consumer) {
    consumer = createConsumer();
    // Extend the connection object (see extensions code below)
    Object.assign(consumer.connection, connectionExtension);
    Object.assign(consumer.connection.events, connectionEventsExtension);
  }

  return consumer;
}

// Msgpack support
// Patches this file: https://github.com/rails/rails/blob/main/actioncable/app/javascript/action_cable/connection.js

// Replace JSON protocol with msgpack
const supportedProtocols = [
  "actioncable-v1-msgpack"
]

const protocols = supportedProtocols
const { message_types } = INTERNAL

const connectionExtension = {
  // We have to override the `open` function, since we MUST provide custom WS sub-protocol
  open() {
    if (this.isActive()) {
      logger.log(`Attempted to open WebSocket, but existing socket is ${this.getState()}`)
      return false
    } else {
      logger.log(`Opening WebSocket, current state is ${this.getState()}, subprotocols: ${protocols}`)
      if (this.webSocket) { this.uninstallEventHandlers() }
      this.webSocket = new adapters.WebSocket(this.consumer.url, protocols)
      this.webSocket.binaryType = "arraybuffer"
      this.installEventHandlers()
      this.monitor.start()
      return true
    }
  },
  isProtocolSupported() {
    return supportedProtocols[0] == this.getProtocol()
  },
  send(data) {
    if (this.isOpen()) {
      const encoded = msgpack.encode(data);
      this.webSocket.send(encoded)
      return true
    } else {
      return false
    }
  }
}

// Incoming messages are handled by the connection.events.message function.
// There is no way to patch it, so, we have to copy-paste :(
const connectionEventsExtension = {
  message(event) {
    if (!this.isProtocolSupported()) { return }
    const {identifier, message, reason, reconnect, type} = msgpack.decode(new Uint8Array(event.data))
    switch (type) {
      case message_types.welcome:
        this.monitor.recordConnect()
        return this.subscriptions.reload()
      case message_types.disconnect:
        logger.log(`Disconnecting. Reason: ${reason}`)
        return this.close({allowReconnect: reconnect})
      case message_types.ping:
        return this.monitor.recordPing()
      case message_types.confirmation:
        return this.subscriptions.notify(identifier, "connected")
      case message_types.rejection:
        return this.subscriptions.reject(identifier)
      default:
        return this.subscriptions.notify(identifier, "received", message)
    }
  },
};
```

🚧 _The work on the AnyCable JavaScript library is in progress. Stay tuned!_

> See the [demo](https://github.com/anycable/anycable_rails_demo/pull/17) of using Msgpack in a Rails project with AnyCable Rack server.

## Protobuf

We squeeze a bit more space by using Protocol Buffers. AnyCable uses the following schema:

```proto
syntax = "proto3";

package action_cable;

enum Type {
  no_type = 0;
  welcome = 1;
  disconnect = 2;
  ping = 3;
  confirm_subscription = 4;
  reject_subscription = 5;
}

enum Command {
  unknown_command = 0;
  subscribe = 1;
  unsubscribe = 2;
  message = 3;
}

message Message {
  Type type = 1;
  Command command = 2;
  string identifier = 3;
  // Data is a JSON encoded string.
  // This is by Action Cable protocol design.
  string data = 4;
  // Message has not structure.
  // We use Msgpack to encode/decode it.
  bytes message = 5;
  string reason = 6;
  bool reconnect = 7;
}
```

Both incoming and outgoing message MUST be encoded as `action_cable.Message` type.

Note that `Message.message` field has the `bytes` type. This field carries the information sent from a server to clients,
which could be of any form. We Msgpack to encode/decode this data. Thus, AnyCable Protobuf protocol is actually a mix of Protobufs and Msgpack.

## Formats comparison

Here is the in/out traffic comparison:

Encoder | Sent | Rcvd
--------|------|-------
protobuf | 315.32MB  | 327.1KB
msgpack  | 339.58MB  | 473.6KB
json     | 502.45MB  | 571.8KB

The data above were captured while running a [websocket-bench][] benchmark with the following parameters:

```sh
websocket-bench broadcast ws://0.0.0.0:8080/cable —server-type=actioncable —origin http://0.0.0.0 —sample-size 100 —step-size 1000 —total-steps 5 —steps-delay 2 —wait-broadcasts=5 —payload-padding=100
```

**NOTE:** The numbers above depend on the messages structure. Binary formats are more efficient for _objects_ (JSON-like) and less efficient when you broadcast long strings (e.g., HTML fragments).

Here is the encode/decode speed comparison:

Encoder | Decode (ns/op) | Encode (ns/op)
--------|------|-------
protobuf (base) | 425  | 1153
msgpack (base) | 676  | 1512
json (base)     | 1386  | 1266
||
protobuf (long) | 479  | 2370
msgpack (long) | 763  | 2506
json (long)   | 2457  | 2319

Where base payload is:

```json
{
  "command": "message",
  "identifier": "{\"channel\":\"test_channel\",\"channelId\":\"23\"}",
  "data": "hello world"
}
```

And the long one is:

```json
{
  "command": "message",
  // x10 means repeat string 10 times
  "identifier": "{\"channel\":\"test_channel..(x10)\",\"channelId\":\"123..(x10)\"}",
  // message is the base message from above
  "message": {
    "command": "message",
    "identifier": "{\"channel\":\"test_channel\",\"channelId\":\"23\"}",
    "data": "hello world"
  }
}
```

[websocket-bench]: https://github.com/anycable/websocket-bench
