# Pusher Protocol Support

AnyCable supports [Pusher protocol](https://pusher.com/docs/channels/library_auth_reference/pusher-websockets-protocol) meaning that it can be used as a drop-in replacement for Pusher (or another Pusher-speaking WebSocket server).

## Configuration

To enable Pusher compatibility mode, you must specify the following parameters:

```sh
$ anycable-go --pusher_app_id=my-test-id --pusher_app_key=my-app-key --pusher_secret=push-secret

...
2025-07-25 01:01:22.826 INF Handle Pusher WebSocket connections at http://localhost:8080/app/my-app-key nodeid=7PKs0u
2025-07-25 01:01:22.826 INF Handle Pusher events s at http://localhost:8080/apps/my-test-id/events nodeid=7PKs0u

```

In the logs, you will see the Pusher WebSocket and HTTP endpoints (see the example above).

Configure Pusher client library and a server-side SDKs accordingly.

## Compatibility

AnyCable doesn't aim to provide 100% compatibility with the current or future Pusher version. The primary purpose of this compatibility layer is to allow applications to gradually switch from Pusher (or its alternative) to AnyCable protocol and benefit from the features provided by it (such as [reliable streams](./reliable_streams.md) _aka_ streams history).

Below you can find the list of support (and not) features:

| Feature | Support Status | Notes |
|---------|----------------|-------|
| Public channels | ✅ | |
| Private channels | ✅ | |
| Client events | ✅ | |
| Presence channels | ✅ | |
| Authentication (`pusher:signin`) |❓| What is if for? |
| Watchlist Events | ❌ | Not planned |
| Webhooks | ❌ | Not planned yet |
| `POST /events` | ✅ |  |
| `POST /batch_events` | ⚙️ | Can be added if needed |
| HTTP API | ❌ | Not planned yet |
| Encrypted channels |❓|  |
