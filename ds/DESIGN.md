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
| GET long-poll (`live=long-poll`) | **WIP** | Parameter validation is in place, but the handler falls back to catch-up mode. A dedicated long-poll loop with timeout/back-off is being implemented. |
| GET SSE (`live=sse`) | **WIP** | SSE transport scaffolding exists (`Connection`, `Encoder`), yet the handler does not expose live subscriptions. Implementation in progress. |
| Cursor support (`Stream-Cursor`) | **Supported** | Server generates time-based cursors for CDN collapsing per DS spec §8.1. Client-provided cursors are used to ensure monotonic progression. |
| Offset format | **Supported** | Offsets are opaque strings produced by `EncodeOffset` (`<offset>::<epoch>`). Reserved offsets `-1` (`StartOffset`) and `now` are honored by `DecodeOffset`. |
| Content negotiation | **JSON only** | Only `application/json` streams are supported; other content types return an error. |

## Write Durable Streams compatibility

Not planned yet.

## Authentication and authorization (read path)

1. `DSHandler` extracts information via `server.NewRequestInfo` and builds a temporary AnyCable session through `NewDSSession`.
2. The session piggybacks on Action Cable authentication (`node.Node.Authenticate`). Non-live requests are created with `node.AsIdleSession` to avoid hub registration overhead.
3. Authentication failures or `common.FAILURE` results produce HTTP 401; canceled authentications map to HTTP 410.
4. On success, the session is trusted to read any stream. A follow-up iteration must enforce stream-level authorization (e.g., via RPC or policy evaluation) before history is served.
5. Cross-origin access uses `Config.AllowedOrigins`; `server.WriteCORSHeaders` emits CORS headers for both simple and preflight requests.

> ⚠️ **Security Note**: The current implementation does NOT enforce stream-level authorization. Any authenticated user can read any stream.

## Read architecture overview

Durable Streams read requests are handled through a thin HTTP façade backed by the existing AnyCable node and broker:

```text
Client (DS SDK)
    |  GET /ds/<stream>?offset=<opaque>&cursor=<token>
    v
DSHandler (HTTP)
    |-- StreamParamsFromReq -> validate path, offset, live mode
    |-- NewDSSession -> node.NewSession -> controller.Authenticate
    |-- fetchHistory -> broker.HistorySince/HistoryFrom
    v
HTTP Response (JSON array body + DS headers)
```

### Read components

- `Config`: toggles DS support, default path `/ds`, and optional origin restrictions.
- `StreamParams`: consolidates the stream path, raw/decoded offsets, epoch, cursor, and live mode.
- `NewDSSession`: bridges DS HTTP requests and AnyCable authentication.
- `Connection`: wraps `http.ResponseWriter` for SSE (future live support) with backlog buffering.
- `Encoder`: converts Action Cable `Reply` messages into DS SSE frames while preserving offsets and cursor metadata.
- `fetchHistory`: retrieves broker history using the decoded offset.
- `GenerateCursor`: generates time-based cursors for CDN collapsing per DS spec §8.1.

## Catch-up request flow

1. Client issues `GET /ds/:stream?offset=...`.
2. `StreamParamsFromReq` normalizes input, enforces live mode requirements, and decodes offsets.
3. The request is authenticated; on success, the handler fetches history via `fetchHistory`.
4. Messages are truncated to `defaultMaxMessages` (100), decoded into a JSON array, and written as the response body.
5. `Stream-Next-Offset` is derived from the last message (or echoes the provided offset when no messages are returned).
6. `Stream-Cursor` is generated based on time intervals for CDN collapsing.
7. `Stream-Up-To-Date` signals the end of available history when fewer than 100 messages are returned.

## Live read scaffolding (WIP)

> **Note**: Live mode support is currently under development.

- Query validation ensures `offset` is provided when `live` is specified (`long-poll` or `sse`).
- Stubbed `handleLongPoll` and `handleSSE` outline headers, keep-alive, and backlog flushing requirements.
- `Connection.Established` handles deferred writes once the SSE handshake is complete.
- Metrics counters/gauges for live modes are registered but not incremented yet.

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

- `EncodeOffset(offset, epoch)` produces opaque strings combining numeric offset and broker epoch.
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
| Invalid request | 400 Bad Request | Malformed offset, missing parameters, invalid live mode |
| Unauthenticated | 401 Unauthorized | Authentication failed or rejected |
| Canceled authentication | 410 Gone | Authentication was canceled by the controller |
| Stale offset / missing data | 410 Gone | Offset before earliest retained position or stream data unavailable |
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
- `ds_poll_clients_total` / `ds_poll_clients_num`: reserved for long-poll tracking (WIP).
- `ds_sse_clients_total` / `ds_sse_clients_num`: reserved for SSE tracking (WIP).

### Configuration knobs

- `ds.enabled` (default `false`) toggles the handler.
- `ds.path` sets the mount point (default `/ds`).
- `AllowedOrigins` governs CORS in CLI/environment settings.

### Response headers

- `X-AnyCable-Version`: Server version for debugging.
- `Stream-Next-Offset`: Next offset for subsequent reads.
- `Stream-Cursor`: Cursor for CDN collapsing.
- `Stream-Up-To-Date`: Present when response includes all available data.

## Future enhancements

### Read roadmap

- Complete long-poll implementation with timeout/heartbeat and graceful shutdown.
- Finish the SSE pipeline (subscriptions, initial replay via `Encoder`, disconnect handling).
- Enforce per-stream authorization before history retrieval.
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
