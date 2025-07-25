# Pusher Protocol Support

[Pusher Documentation](https://pusher.com/docs/channels/library_auth_reference/pusher-websockets-protocol/).

## Architecture Overview

The Pusher protocol support consists of four main components:

1. **Encoder** (`encoder.go`): Handles protocol translation between Pusher and AnyCable formats.
2. **Controller** (`controller.go`): Manages Pusher-specific business logic for subscriptions and channels.
3. **Executor** (`executor.go`): Wraps the main AnyCable executor with Pusher-specific authorization logic.
4. **Verifier** (`verifier.go`): Implements Pusher signature verification logic.

## Message Flow Architecture

### Incoming Message Flow (Client → AnyCable)

Let's consider a `pusher:subscribe` command as an example:

1. **Client sends Pusher JSON message**: Raw WebSocket frame with Pusher protocol format.
2. **Encoder.Decode()**: Converts Pusher message format to AnyCable's `common.Message`.
3. **Executor.HandleCommand()**\*: Verifies signature for private and presence channel subsciptions.
4. **Controller.Subscribe**: Configure the subscription details (whispering, presence, etc.)

\* We need the executor wrapper to verify the `auth` payload without keeping it around in the identifier (the only _storage_ we have at the protocol level), because other commands (`unsubscribe`, client events) do not include the auth part.

### Broadcast Flow

AnyCable supports handling Pusher events at `/app/:id/events`:

1. Application sends a POST request:

```json
{"name":"notification","channels":["user-notifications"],"data":"{\"title\":\"New message\",\"body\":\"You have mail\"}"}
```

1. AnyCable translates it into a broadcast message:

```json
{
  "stream": "user-notifications",
  "data": "{\"event\":\"notification\",\"data\":\"\\\"{\\\"title\\\":\\\"New message\\\",\\\"body\\\":\\\"You have mail\\\"}\\\"\"}"
}
```

3. Under the hood, AnyCable creates a `common.Reply` struct:

```go
common.Reply{
	Identifier: `{"channel":"$pusher","stream_name":"user-notifications"}`,
	Message: map[string]interface{}{
		"event": "notification",
		"data": {"title": "New message", "body": "You have mail"}
	}
}
```

2. `Encoder.Encode()` converts to Pusher format:

```json
{
  "event": "notification",
  "channel": "user-notifications",
  "data": {
    "title": "New message",
    "body": "You have mail"
  }
}
```

## Protocol Translation

### Pusher to AnyCable Message Mapping

The encoder handles the following Pusher event types:

- `pusher:subscribe` → AnyCable `subscribe` command
- `pusher:unsubscribe` → AnyCable `unsubscribe` command
- `pusher:ping` → AnyCable `ping` command
- `pusher:pong` → AnyCable `pong` command
- Client events -> AnyCable `whisper` command

### AnyCable to Pusher Message Mapping

- `common.WelcomeType` → `pusher:connection_established`
- `common.ConfirmedType` → `pusher_internal:subscription_succeeded`
- `common.RejectedType` → `pusher:error` (code 4009)
- `common.DisconnectType` → `pusher:error` (code 4200 or 4009)
- `common.PingType` → `pusher:ping`
- `common.PongType` → `pusher:pong`

## Channel Identifier Translation

Pusher channels are mapped to AnyCable identifiers using a special `$pusher` channel wrapper:

**Pusher message:**

```json
{
  "event": "pusher:subscribe",
  "data": {
    "channel": "chat-room"
  }
}
```

**AnyCable identifier:**

```json
{
  "channel": "$pusher",
  "stream": "chat-room"
}
```
