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

If you use AnyCable Pro, you can also use the Redis broker:

```sh
anycable-go --broker=redis
```

**NOTE:** Redis broker is the only broker that works in the cluster mode.

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

Alternatively to joining and leaving the channel's presence set from the client, you can use control the presence behaviour from the server-side when using channels.

For that, you can provide presence commands (`join` and `leave`) in subscription callbacks
and channel actions.

### Ruby on Rails integration

Here is a quick overview of using Presence API in Rails' Action Cable:

```ruby
class ChatChannel < ApplicationCable::Channel
  def subscribed
    room = Chat::Room.find(params[:id])

    stream_for room

    join_presence(id: current_user.id, info: {name: current_user.name})
  end
end
```

The full documentation could be found [here](https://docs.anycable.io/edge/rails/extensions?id=presence-tracking).

### JavaScript integration

> 🚧 Presence support in [anycable-serverless-js](https://github.com/anycable/anycable-serverless-js) is coming soon.

## Presence for Hotwire

> Read more in the ["Simple Declarative Presence for Hotwire apps with AnyCable"](https://evilmartians.com/chronicles/simple-declarative-presence-for-hotwire-apps-with-anycable) blog post.

For Hotwire applications, our `@anycable/turbo-stream` package (>= 0.8.0) provides a custom `<turbo-cable-presence-source>` element to add presence information on the page without needing to write any custom client-side code.

The complete documentation is coming soon; for now, you can check out this [example](https://github.com/anycable/anycasts_demo/pull/17).

## Presence API

> 🚧 Presence REST API is to be implemented. You can only use the presence API via the WebSocket connection. Please, reach out to us if you need to use it and share your use cases.

## Presence webhooks

> 🚧 Presence webhooks are to be implemented, too. Please, reach out to us if you need to use it and share your use cases.
