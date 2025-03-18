# Getting Started with AnyCable

AnyCable is a language-agnostic real-time server focused on performance and reliability written in Go.

> The quickest way to get AnyCable is to use our managed (and free) solution: [plus.anycable.io](https://plus.anycable.io)

## Installation

The easiest way to install AnyCable-Go is to [download](https://github.com/anycable/anycable/releases) a pre-compiled binary (for versions < 1.6.0 use our [legacy repository](https://github.com/anycable/anycable-go/releases)).

MacOS users could install it with [Homebrew](https://brew.sh/)

```sh
brew install anycable-go
```

Arch Linux users can install [anycable-go package from AUR](https://aur.archlinux.org/packages/anycable-go/).

### Via NPM

For JavaScript projects, there is also an option to install AnyCable-Go via NPM:

```sh
npm install --save-dev @anycable/anycable-go
pnpm install --save-dev @anycable/anycable-go
yarn add --dev @anycable/anycable-go

# and run as follows
npx anycable-go
```

**NOTE:** The version of the NPM package is the same as the version of the AnyCable server binary (which is downloaded automatically on the first run).

## Usage

After installation, you can run AnyCable as follows:

```sh
$ anycable-go

2024-03-06 13:38:07.545 INF Starting AnyCable 1.6.0-4f16b99 (pid: 8289, open file limit: 122880, gomaxprocs: 8) nodeid=hj2mXN
...
2024-03-06 13:38:56.490 INF RPC controller initialized: localhost:50051 (concurrency: 28, impl: grpc, enable_tls: false, proto_versions: v1) nodeid=FlCtwf context=rpc
```

By default, AnyCable tries to connect to a gRPC server listening at `localhost:50051` (the default host for the Ruby gem).

AnyCable is designed as a logic-less proxy for your real-time features relying on a backend server to authenticate connections, authorize subscriptions and process incoming messages. That's why our default configuration assumes having an RPC server to handle all this logic.

You can read more about AnyCable RPC in the [corresponding documentation](./rpc.md).

### Standalone mode (pub/sub only)

For pure pub/sub functionality, you can use AnyCable in a standalone mode, without any RPC servers. For that, you must configure the following features:

- [JWT authentication](./jwt_identification.md) or disable authentication completely (`--noauth`). **NOTE:** You can still add minimal protection via the `--allowed_origins` option (see [configuration](./configuration.md#primary-settings)).

- Enable [signed streams](./signed_streams.md) or allow public streams via the `--public_streams` option.

There is also a shortcut option `--public` to enable both `--noauth` and `--public_streams` options. **Use it with caution**.

You can also explicitly disable the RPC component by specifying the `--norpc` option.

Thus, to run AnyCable real-time server in an insecure standalone mode, use the following command:

```sh
$ anycable-go --public

2024-03-06 14:00:12.549 INF Starting AnyCable 1.6.0-4f16b99 (pid: 17817, open file limit: 122880, gomaxprocs: 8) nodeid=wAhWDB
2024-03-06 14:00:12.549 WRN Server is running in the public mode nodeid=wAhWDB
...
```

To secure access to AnyCable server, specify either the `--jwt_secret` or `--streams_secret` option. There is also the `--secret` shortcut:

```sh
anycable-go --secret=VERY_SECRET_VALUE --norpc
```

Read more about pub/sub mode in the [signed streams documentation](./signed_streams.md).

### Connecting to AnyCable

AnyCable uses the [Action Cable protocol][protocol] for client-server communication. We recommend using our official [JavaScript client library][anycable-client] for all JavaScript/TypeScript runtimes:

```js
import { createCable } from '@anycable/web'

const cable = createCable(CABLE_URL)

const subscription = cable.subscribeTo('ChatChannel', { roomId: '42' })

const _ = await subscription.perform('speak', { msg: 'Hello' })

subscription.on('message', msg => {
  if (msg.type === 'typing') {
    console.log(`User ${msg.name} is typing`)
  } else {
    console.log(`${msg.name}: ${msg.text}`)
  }
})
```

**Note**: The snippet above assumes having a "ChatChannel" defined in your application (which is connected to AnyCable via RPC).

You can also use:

- Third-party Action Cable-compatible clients.

- EventSource (Server-Sent Events) connections ([more info](./sse.md)).

- Custom WebSocket clients following the [Action Cable protocol][protocol].

AnyCable Pro also supports:

- Apollo GraphQL WebSocket clients ([more info](./apollo.md))

- HTTP streaming (long-polling) ([more info](./long_polling.md))

- OCPP WebSocket clients ([more info](./ocpp.md))

### Broadcasting messages

Finally, to broadcast messages to connected clients via the name pub/sub streams, you can use one of the provided [broadcast adapters](./broadcasting.md).

[anycable-client]: https://github.com/anycable/anycable-client
[protocol]: ../misc/action_cable_protocol.md
