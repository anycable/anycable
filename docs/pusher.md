# Pusher Compatibility

AnyCable supports [Pusher protocol](https://pusher.com/docs/channels/library_auth_reference/pusher-websockets-protocol) meaning that it can be used as a drop-in replacement for Pusher (or another Pusher-speaking WebSocket server such as Laravel Reverb, Soketi, etc.).

## Configuration

To enable Pusher compatibility mode, you must specify the following parameters:

- `--pusher_app_id` (or `ANYCABLE_PUSHER_APP_ID`): Pusher application ID
- `--pusher_app_key` (or `ANYCABLE_PUSHER_APP_KEY`): Pusher application key
- `--pusher_secret` (or `ANYCABLE_PUSHER_SECRET`): Pusher secret for signing (falls back to `--secret` if not specified)

Example:

```sh
$ anycable-go --pusher_app_id=my-app-id --pusher_app_key=my-app-key --pusher_secret=my-secret

...
INFO 2025-01-20 12:00:00.000 INF Handle Pusher WebSocket connections at http://localhost:8080/app/my-app-key
INFO 2025-01-20 12:00:00.000 INF Handle Pusher API requests at http://localhost:8080/apps/my-app-id/
```

In the logs, you will see the Pusher WebSocket and HTTP API endpoints. Configure your Pusher client library and server-side SDKs accordingly.

### Dedicated API port

By default, the Pusher HTTP API is served on the same port as WebSocket connections. You can configure a separate port for the HTTP API using the `--pusher_api_port` option:

```sh
$ anycable-go \
  --pusher_app_id=my-app-id \
  --pusher_app_key=my-app-key \
  --pusher_secret=my-secret \
  --pusher_api_port=8081

...
INFO Handle Pusher WebSocket connections at http://localhost:8080/app/my-app-key
INFO Handle Pusher API requests at http://localhost:8081/apps/my-app-id/
```

This is useful when you want to expose the HTTP API only within a private network while keeping WebSocket connections publicly accessible.

## HTTP API

AnyCable implements a subset of Pusher HTTP API for server-to-server communication. All API requests require authentication using the [Pusher signature scheme](https://pusher.com/docs/channels/library_auth_reference/rest-api/#authentication).

### Trigger events

**Endpoint:** `POST /apps/{app_id}/events`

Broadcast an event to one or more channels:

```sh
curl -X POST "http://localhost:8080/apps/my-app-id/events?auth_key=my-app-key&auth_timestamp=$(date +%s)&auth_version=1.0&body_md5=$(echo -n '{"name":"my-event","channel":"my-channel","data":"{}"}' | md5sum | cut -d' ' -f1)&auth_signature=<signature>" \
  -H "Content-Type: application/json" \
  -d '{"name":"my-event","channel":"my-channel","data":"{}"}'
```

We recommend using official Pusher server SDKs which handle authentication automatically.

### Get channel users

**Endpoint:** `GET /apps/{app_id}/channels/{channel_name}/users`

Retrieve the list of users subscribed to a presence channel:

```sh
curl "http://localhost:8080/apps/my-app-id/channels/presence-my-channel/users?auth_key=my-app-key&auth_timestamp=$(date +%s)&auth_version=1.0&auth_signature=<signature>"
```

**Response:**

```json
{
  "users": [
    {"id": "user-1"},
    {"id": "user-2"}
  ]
}
```

**Notes:**

- This endpoint only works with presence channels (channels prefixed with `presence-`)
- Returns `400 Bad Request` for non-presence channels
- Returns an empty list for unknown channels

**Example using Pusher Ruby SDK:**

```go
require "pusher"

client = Pusher::Client.new(
  app_id: "my-app-id",
  key: "my-app-key",
  secret: "my-secret"
)

# Request the users for a presence channel:
response = pusher.channel_users("presence-my-channel")

response["users"].each do |user|
  puts "User ID: #{user['id']}"
end
```

## Client configuration

Configure your Pusher client to connect to AnyCable:

### JavaScript

```js
import Pusher from 'pusher-js';

const pusher = new Pusher('my-app-key', {
  wsHost: 'localhost',
  wsPort: 8080,
  forceTLS: false,
  disableStats: true,
  enabledTransports: ['ws', 'wss'],
});
```

### Laravel Echo

```js
import Echo from 'laravel-echo';
import Pusher from 'pusher-js';

window.Pusher = Pusher;

window.Echo = new Echo({
  broadcaster: 'pusher',
  key: 'my-app-key',
  wsHost: 'localhost',
  wsPort: 8080,
  forceTLS: false,
  disableStats: true,
  enabledTransports: ['ws', 'wss'],
});
```

## Compatibility

AnyCable doesn't aim to provide 100% compatibility with the current or future Pusher versions. The primary purpose of this compatibility layer is to allow applications to use AnyCable as a Pusher replacement and, optionally, gradually migrate to the AnyCable protocol to benefit from its features (such as [reliable streams](./reliable_streams.md)).

### Supported features

| Feature | Status | Notes |
|---------|--------|-------|
| Public channels | ✅ | |
| Private channels | ✅ | |
| Presence channels | ✅ | |
| Client events (whispers) | ✅ | |
| `POST /events` | ✅ | Trigger events API |
| `GET /channels/{channel}/users` | ✅ | Get presence channel users |
| `POST /batch_events` | ⚙️ | Can be added if needed |
| Webhooks | ⏳ | Planned |
| Watchlist events | ❌ | Not planned |
| Encrypted channels | ❓ | |
| Authentication (`pusher:signin`) | ❓ | |
