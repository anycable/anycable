# Presence tracking

AnyCable comes with a built-in presence tracking support for your real-time applications. No need to write custom code and deal with storage mechanisms to know who's online in your channels.

## Overview

AnyCable presence allows channel subscribers to share their presence information with other clients and track the changes in the channel's presence set. Presence data can be used to display a list of online users, track user activity, etc.

## Quick start

Presence is a part of the [broker](./broker.md) component, so you must enable it either via the `broker` configuration preset or manually:

```sh
$ anycable-go --presets=broker

# or

$ anycable-go --broker=memory
```

> 🚧 Currently, presence tracking is only supported by the memory broker.

Now, you can use the presence API in your application. For example, using [AnyCable JS client](https://github.com/anycable/anycable-client):

```js
import { createCable } from '@anycable/web'
// or for non-web projects
// import { createCable } from '@anycable/core'

const cable = createCable({protocol: 'actioncable-v1-ext-json'})

const channel = cable.streamFrom('room/42');

// join the channel's presence set
channel.presence.join(user.id, { name: user.name })

// get the current presence state
const presence = await chatChannel.presence.info()

// subscribe to presence events
channel.on("presence", (event) => {
  const { type, info, id } = event

  if (type === "join") {
    console.log(`${info.name} joined the channel`)
  } else if (type === "leave") {
    console.log(`${id} left the channel`)
  }
})
```

## Presence lifecycle

Clients join the presence set explicitly by performing the `presence` command. The `join` event is sent to all subscribers (including the initiator) with the presence information, but only if the **presence ID** (provided by the client) hasn't been registered yet. Thus, multiple sessions with the same ID are treated as a single presence record.

Clients may explicitly leave the presence set by performing the `leave` command or by unsubscribing from the channel. The `leave` event is sent to all subscribers only if no other sessions with the same ID are left in the presence set.

When a client disconnects without explicitly leaving or unsubscribing the channel, it's present information stays in the set for a short period of time. That prevents the burst of `join` / `leave` events when the client reconnects frequently.

## Configuration

You can configure the presence expiration time (for disconnected clients) via the `--presence_ttl` option. The default value is 15 seconds.

## Presence for channels

> 🚧 Presence for channels is to be implemented. You can only use presence with [signed streams](./signed_streams.md) for now.

## Presence API

> 🚧 Presence REST API is to be implemented. You can only use the presence API via the WebSocket connection.

## Presence webhooks

> 🚧 Presence webhooks are to be implemented, too.
