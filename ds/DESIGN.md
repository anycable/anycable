# AnyCable Durable Streams Design

_Disclaimer: this document is written and maintained by 🤖 under humans supervision_.

## Scope and goals

This document describes how AnyCable implements the **read** portion of the Durable Streams (DS) specification (https://github.com/durable-streams/durable-streams).

Write-side semantics (appends, stream creation/upserts, TTL updates) are intentionally out of scope and are not planned for the near future.

> **Note**: AnyCable's DS implementation only supports `application/json` content type. Other content types (e.g., `text/plain`, `application/octet-stream`) are not supported and not planned.

## Read Durable Streams compatibility

| Spec area | Status | Notes |
| --- | --- | --- |
| HEAD metadata | **Partial** | Responds with JSON content type, cache control, CORS headers, and `Stream-Next-Offset`, which is temporarily set to the placeholder value `now` until broker introspection is exposed. |
| GET catch-up reads | **Supported** | Fetches up to 100 persisted messages via `broker.HistorySince`/`HistoryFrom`, encodes them as a JSON array, and sets `Stream-Next-Offset`, `Stream-Cursor`, and `Stream-Up-To-Date`. |
| GET long-poll (`live=long-poll`) | **Supported** | Subscribes to the stream after returning empty catch-up, waits for new messages or timeout (configurable via `poll_interval`, default 10s), returns single message with proper headers. |
| GET SSE (`live=sse`) | **WIP** | SSE encoder exists, but the handler returns 501 Not Implemented. Full subscription pipeline pending. |
| Cursor support (`Stream-Cursor`) | **Supported** | Server generates time-based cursors for CDN collapsing per DS spec §8.1. Client-provided cursors are used to ensure monotonic progression. |
| Offset format | **Supported** | Offsets are opaque strings produced by `EncodeOffset` (`<offset>::<epoch>`). Reserved offsets `-1` (`StartOffset`) and `now` are honored by `DecodeOffset`. |
| Content negotiation | **JSON only** | Only `application/json` streams are supported; other content types return an error. |

## Write Durable Streams compatibility

Not planned yet.

## Authentication and authorization (read path)

1. `DSHandler` extracts information via `server.NewRequestInfo` and builds a `Stream` wrapper through `NewDSSession`.
2. The session is created with mode-specific encoders (`NoopEncoder` for catch-up, `PollEncoder` for long-poll, `SSEEncoder` for SSE) and connection types (`PollConnection` or `sse.Connection`).
3. Sessions are registered directly via `node.Authenticated()` with a DS-specific identifier (`ds::<session_id>`), bypassing the traditional Action Cable authentication flow.
4. Non-live (catch-up) requests use `node.InactiveSession()` to avoid hub registration overhead.
5. Cross-origin access uses `Config.AllowedOrigins`; `server.WriteCORSHeaders` emits CORS headers for both simple and preflight requests.

> ⚠️ **Security Note**: The current implementation does NOT enforce stream-level authorization. Any user can read any stream. JWT authentication integration is planned.

## Read architecture overview

Durable Streams read requests are handled through a thin HTTP façade backed by the existing AnyCable node and broker:

```text
Client (DS SDK)
    |  GET /ds/<stream>?offset=<opaque>&cursor=<token>&live=<mode>
    v
DSHandler (HTTP)
    |-- StreamParamsFromReq -> validate path, offset, live mode
    |-- NewDSSession -> create Stream with mode-specific encoder/connection
    |-- node.Authenticated -> register session
    |
    |-- [catch-up] fetchHistory -> broker.HistorySince/HistoryFrom
    |-- [long-poll] node.Subscribe -> wait for message or timeout
    |-- [sse] (not implemented)
    v
HTTP Response (JSON array body + DS headers)
```

### Read components

- `Config`: toggles DS support, default path `/ds`, poll interval, and optional origin restrictions.
- `StreamParams`: consolidates the stream path/name, raw/decoded offsets, epoch, cursor, and live mode.
- `Stream`: wraps `node.Session`, `StreamParams`, and `node.Connection` to represent a durable stream connection.
- `NewDSSession`: creates the `Stream` with appropriate encoder and connection based on live mode.
- `PollConnection`: wraps `http.ResponseWriter` for long-poll mode, converting encoded messages to HTTP response with headers.
- `NoopEncoder`: used for catch-up requests where message encoding is handled directly.
- `PollEncoder`: encodes messages with offset prepended for header extraction in long-poll mode.
- `SSEEncoder`: converts Action Cable `Reply` messages into DS SSE frames with offset and cursor metadata.
- `fetchHistory`: retrieves broker history using the decoded offset.
- `Stream.NextCursor`: generates time-based cursors for CDN collapsing per DS spec §8.1.

## Catch-up request flow

1. Client issues `GET /ds/:stream?offset=...`.
2. `StreamParamsFromReq` normalizes input, enforces live mode requirements, and decodes offsets.
3. `NewDSSession` creates a `Stream` with `NoopEncoder` and `InactiveSession()` option.
4. The handler fetches history via `fetchHistory`.
5. Messages are truncated to `defaultMaxMessages` (100), decoded into a JSON array, and written as the response body.
6. `Stream-Next-Offset` is derived from the last message (or echoes the provided offset when no messages are returned).
7. `Stream-Cursor` is generated via `Stream.NextCursor()` based on time intervals for CDN collapsing.
8. `Stream-Up-To-Date` signals the end of available history when fewer than 100 messages are returned.

## Long-poll request flow

1. Client issues `GET /ds/:stream?offset=<offset>&live=long-poll`.
2. `StreamParamsFromReq` validates that offset is provided (required for live mode).
3. `NewDSSession` creates a `Stream` with `PollEncoder` and `PollConnection`.
4. The handler first attempts catch-up via `fetchHistory`.
5. If messages exist, they are returned immediately as a JSON array with headers.
6. If no messages are available:
   - Session subscribes to the stream via `node.Subscribe` using a pub/sub channel identifier.
   - Handler enters a select loop waiting for:
     - **Message received**: `PollConnection.Write` is triggered, which sets headers and writes the response.
     - **Timeout**: After `poll_interval` seconds (default 10), returns 204 No Content.
     - **Client disconnect**: Request context cancellation terminates the handler.
     - **Server shutdown**: Returns 410 Gone.
7. `PollEncoder` prepends offset to message data; `PollConnection` parses this to set `Stream-Next-Offset` header.
8. Response includes single message wrapped in JSON array with `Stream-Up-To-Date: true`.

## SSE scaffolding (WIP)

> **Note**: SSE mode support is currently under development.

- Query validation ensures `offset` is provided when `live=sse` is specified.
- `SSEEncoder` is implemented to produce SSE-formatted events with data and control frames.
- Handler currently returns 501 Not Implemented for SSE requests.
- `sse.Connection` from the shared SSE package will be used for the connection wrapper.
- Metrics counters/gauges for SSE mode are registered but not incremented yet.

## Offset semantics

### Offset format

AnyCable uses a proprietary offset format that combines a numeric offset with a broker epoch:

```
<offset>::<epoch>
```

For example: `42::abc123` represents offset 42 in epoch `abc123`.

> **Important**: Per the DS specification, offsets are **opaque tokens**. Clients MUST NOT parse or interpret offset values. They should only store and echo them back to the server for subsequent requests.

### Reserved offsets

- `-1` (`StartOffset`): Read from the beginning of the stream.
- `now`: Skip to the current tail position (returns empty data with the tail offset).
- `0` or empty string: Treated as start-of-stream.

### Encoding and decoding

- `EncodeOffset(offset, epoch)` produces opaque strings combining numeric offset and broker epoch. Returns `"0"` for empty epoch (empty stream).
- `DecodeOffset(offsetStr)` parses the opaque string back into offset number and epoch. It tolerates special offsets (`-1`, `now`, `0`, empty) and returns `(0, "")` for start-of-stream semantics.

### Next offset behavior

When returning messages, `Stream-Next-Offset` is set to the offset of the last returned message. This is consistent with AnyCable's broker semantics where the offset points to the last read position.

## Broker interaction

- The DS layer leverages AnyCable's broker history; no additional storage is introduced.
- Start-of-stream reads call `HistorySince(stream, 0)`.
- Epoch-aware resumes call `HistoryFrom(stream, epoch, offset)`.
- Broker TTL/limit configurations control retention; the handler does not add pagination beyond the soft cap.

## HTTP status codes

| Condition | HTTP Status | Notes |
| --- | --- | --- |
| Success | 200 OK | Data returned (or empty array for empty streams) |
| No content (long-poll timeout) | 204 No Content | Poll interval elapsed with no new messages |
| Invalid request | 400 Bad Request | Malformed offset, missing parameters, invalid live mode |
| Unauthenticated | 401 Unauthorized | Authentication failed or rejected |
| Stale offset / missing data | 410 Gone | Offset before earliest retained position or stream data unavailable |
| Server shutdown (long-poll) | 410 Gone | Server is shutting down during long-poll wait |
| Not implemented | 501 Not Implemented | SSE mode requested (currently unsupported) |
| Internal error | 500 Internal Server Error | Unexpected server errors |

## Caching and CDN support

### Cache-Control headers

- **HEAD requests**: `Cache-Control: no-store` (tail offset should not be cached)
- **Catch-up reads**: `Cache-Control: public, max-age=60, stale-while-revalidate=300`

> **Note**: The current implementation uses `public` caching. For streams containing user-specific or confidential data, consider implementing authentication-aware cache keys at the CDN level.

### Cursor-based CDN collapsing

Per DS spec §8.1, the server generates `Stream-Cursor` headers to enable CDN request collapsing:

1. Time is divided into 20-second intervals from a fixed epoch (October 9, 2024 00:00:00 UTC).
2. The cursor value is the current interval number as a decimal string.
3. If a client provides a cursor >= current interval, the server adds jitter to ensure monotonic progression.
4. Clients should echo the received cursor in subsequent requests via the `cursor` query parameter.

This mechanism prevents infinite CDN cache loops where clients receive the same cached empty response indefinitely.

## Observability and configuration

### Metrics

The following metrics are provided:

- `ds_requests_total`: total DS GET/HEAD requests.
- `ds_poll_requests_total`: total long-poll requests initiated.
- `ds_poll_clients_num`: current number of active long-poll clients waiting.
- `ds_sse_requests_total`: reserved for SSE tracking (WIP).
- `ds_sse_clients_num`: reserved for SSE tracking (WIP).

### Configuration knobs

- `ds.enabled` (default `false`) toggles the handler.
- `ds.path` sets the mount point (default `/ds`).
- `ds.poll_interval` sets the long-poll timeout in seconds (default `10`).
- `AllowedOrigins` governs CORS in CLI/environment settings.

### Response headers

- `X-AnyCable-Version`: Server version for debugging.
- `Stream-Next-Offset`: Next offset for subsequent reads.
- `Stream-Cursor`: Cursor for CDN collapsing.
- `Stream-Up-To-Date`: Present when response includes all available data.

## Future enhancements

### Read roadmap

- Finish the SSE pipeline (subscriptions, initial replay via `SSEEncoder`, disconnect handling, keep-alive).
- Implement JWT authentication for stream access.
- Enforce per-stream authorization before history retrieval (signed streams).
- Wire broker metadata into HEAD responses (`Stream-Next-Offset`, history size, retention info).
- Emit rich observability (structured logs, per-mode metrics, error counters).
- Parameterize limits (max messages, payload size) and consider pagination tokens.

### Write considerations (deferred)

- Expose append/merge endpoints with optimistic concurrency based on `Stream-Seq`.
- Support TTL/expiry updates aligned with DS creation semantics.
- Manage stream lifecycles (creation/deletion) if/when writes become a priority.

## References

- Durable Streams specification: https://github.com/durable-streams/durable-streams
- Durable Streams protocol: https://github.com/durable-streams/durable-streams/blob/main/PROTOCOL.md