# PostgreSQL Signalling Implementation Plan

This document captures the proposed direction for the PostgreSQL signalling PR.
All PostgreSQL signalling code in this PR is new to AnyCable, so the goal is a
clean final shape rather than preserving the first local implementation.

## Design Stance

- Polling is the source of truth. `LISTEN/NOTIFY` only wakes polling loops and
  never carries durable delivery payloads.
- Do not create one PostgreSQL `LISTEN` channel per stream. Use two stable
  wake-up channels and include enough payload metadata for nodes to decide
  whether they need to catch up.
- Treat app-to-server broadcasts and node-to-node fanout as two logical links
  with different delivery semantics.
- Keep this PR focused on signalling. Broker/history support can use the same
  tables later, but this PR should not claim `HistoryFrom`, `HistorySince`, or
  `Peak` support.
- Guarantee ordering per stream, not globally across unrelated streams.

## Target Shape

![AnyCable PostgreSQL data model](images/postgres-signalling-data-model.png)

### Server-owned schema

AnyCable Go owns the schema required by PostgreSQL-backed components:

- create or actualize the schema automatically when a PostgreSQL-backed
  component is active;
- fail startup if schema management is enabled and the schema cannot be created
  or validated;
- provide `postgres_ensure_schema` as a concise opt-out flag for deployments
  that manage schema externally;
- keep publishing applications behind SQL functions instead of requiring them
  to know table layouts.

No schema marker table is required for the first version. Startup should run
idempotent DDL for the required tables, indexes, triggers, and SQL functions,
then interrogate PostgreSQL catalogs to validate the resulting shape. This keeps
the table surface to the two delivery tables: `anycable_broadcasts` and
`anycable_pubsub`.

The proposed flag follows the existing `postgres_*` configuration convention:
`--postgres_ensure_schema=false` skips DDL creation/actualization. Startup still
interrogates PostgreSQL catalogs and fails when the schema shape is incompatible.

### App-to-server queue

`anycable_broadcasts` is a single-consumer queue for publications created by
applications:

- rows include `stream` as a first-class column, not only inside JSON payload;
- AnyCable nodes poll with `FOR UPDATE SKIP LOCKED`;
- exactly one node claims and processes a row;
- successful rows are acked;
- failed rows are released for retry or left with failure details after attempts
  are exhausted;
- the behavior for exhausted rows is controlled by
  `postgres_exhausted_broadcast_policy`;
- `NOTIFY` only wakes nodes after inserts.

This keeps the existing broadcaster shape, but makes the stream visible to SQL
so the claim query can preserve per-stream order.

`postgres_exhausted_broadcast_policy` should support:

- `skip`: keep failure details for inspection, but unblock later same-stream
  rows after the older row exhausts attempts;
- `block`: keep the exhausted row as an ordering barrier until an operator
  resolves or removes it.

`skip` is the pragmatic default because it avoids one poison row stopping a
stream forever. `block` is available for deployments that prefer strict
same-stream sequence handling after unrecoverable failures.

### Node-to-node fanout log

`anycable_pubsub` is a fanout catch-up log:

- each interested AnyCable node may read the same row;
- rows are fetched by stream cursor, not claimed;
- notification payloads include the changed stream name;
- nodes ignore notifications for streams with no local subscribers;
- wakeups can be coalesced before querying;
- fallback ticks batch all subscribed stream cursors instead of issuing one
  query per stream.

This keeps PostgreSQL from acting like Redis with thousands of dynamic
`LISTEN`s, while still avoiding the current N-per-stream polling loop.

## Dataflow

![AnyCable PostgreSQL dataflow](images/postgres-signalling-dataflow.png)

1. A publishing application calls `anycable_publish(stream, payload, meta)`.
2. PostgreSQL inserts an `anycable_broadcasts` row and sends a wake-up
   notification on the app-to-server channel.
3. AnyCable nodes poll the broadcast queue. The claim query uses
   `FOR UPDATE SKIP LOCKED` and only claims rows that have no older unfinished
   row for the same stream.
4. The winning node handles the broadcast through the broker path.
5. The broker writes a node-to-node fanout row to `anycable_pubsub`.
6. PostgreSQL sends a wake-up notification whose payload includes the stream
   name on the node-to-node channel.
7. Nodes subscribed to that stream batch-fetch rows newer than their local
   cursor and deliver them in stream order.
8. Periodic polling remains the correctness fallback if notifications are
   missed, delayed, or coalesced.

## Ordering Guarantee

The practical guarantee should be per-stream ordering. Global ordering across
unrelated streams is not required and would unnecessarily serialize work.

The app-to-server claim query should skip rows that have an older unfinished row
for the same stream:

```sql
WITH candidates AS (
  SELECT broadcasts.id
  FROM anycable_broadcasts broadcasts
  WHERE broadcasts.attempts < $attempts_limit
    AND (
      broadcasts.claimed_at IS NULL
      OR broadcasts.claimed_at < now() - ($claim_timeout::bigint * interval '1 second')
    )
    AND NOT EXISTS (
      SELECT 1
      FROM anycable_broadcasts older
      WHERE older.stream = broadcasts.stream
        AND older.id < broadcasts.id
        AND older.attempts < $attempts_limit
    )
  ORDER BY broadcasts.id
  LIMIT $batch_limit
  FOR UPDATE SKIP LOCKED
)
UPDATE anycable_broadcasts AS broadcasts
SET claimed_by = $node_id,
    claimed_at = now(),
    attempts = broadcasts.attempts + 1,
    last_error = NULL
FROM candidates
WHERE broadcasts.id = candidates.id
RETURNING broadcasts.id, broadcasts.stream, broadcasts.payload, broadcasts.attempts;
```

This allows rows from different streams to be processed concurrently while
serializing rows for the same stream. If an older same-stream row exhausts its
attempts, the configured exhausted-row policy decides whether later rows may
proceed.

For node-to-node fanout, each node should serialize its own polling loop,
deliver rows ordered by `(stream, id)`, and advance a stream cursor only after
the row is delivered. That preserves per-stream order locally without requiring
cross-stream ordering.

## Batched Fanout Polling

The current per-stream loop should be replaced by a batched query. Wake-up
notifications enqueue changed streams; fallback ticks use all subscribed stream
cursors.

One possible shape:

```sql
WITH cursors AS (
  SELECT *
  FROM unnest($streams::text[], $cursors::bigint[]) AS c(stream, cursor)
)
SELECT publications.stream, publications.id, publications.payload
FROM cursors
JOIN LATERAL (
  SELECT id, payload
  FROM anycable_pubsub
  WHERE stream = cursors.stream
    AND id > cursors.cursor
  ORDER BY id
  LIMIT $per_stream_limit
) publications ON true
ORDER BY publications.stream, publications.id;
```

The exact SQL can change during implementation, but the key property is that
AnyCable performs bounded catch-up for a set of streams in one database round
trip instead of one round trip per locally subscribed stream.

## SQL Functions

Publishing applications should call functions instead of inserting rows
directly:

- `anycable_publish(stream, payload, meta default '{}')`
- `anycable_remote_command(payload, meta default '{}')`

The functions should:

- validate required inputs;
- insert into the app-to-server queue;
- trigger a wake-up notification;
- return the created row id for observability.

Payload columns should be `text`. The payload is an opaque serialized AnyCable
message; routing and queue behavior should rely on explicit columns such as
`stream` and claim state, not SQL inspection of the payload body. If future
broker/history work needs structured metadata, add separate metadata columns
rather than making the main payload `jsonb`.

The node-to-node fanout table remains an internal server detail in this PR.

## Implementation Steps

1. Replace external schema validation with server-owned schema ensure plus
   catalog validation.
2. Add `stream` to the app-to-server broadcast queue schema and publishing
   function.
3. Update the broadcast claim query to enforce per-stream ordering.
4. Keep final failed rows claimed for inspection; non-final failures release
   their claim for retry.
5. Use two stable notification channels, one for app-to-server wakeups and one
   for node-to-node fanout wakeups; do not add per-stream `LISTEN`s.
6. Pass notification payloads into adapter wake-up code so pub/sub can enqueue
   changed streams.
7. Replace pub/sub per-stream polling with batched catch-up by stream cursor.
8. Update docs and tests to describe polling as the correctness mechanism and
   notify as latency optimization.

## Test Plan

Core coverage:

- schema ensure is idempotent and runs automatically when a PostgreSQL-backed
  component is active;
- opt-out skips schema creation/modification, while catalog validation still
  fails clearly for incompatible externally managed schemas;
- required tables, functions, indexes, and triggers are validated without a
  schema marker/version table;
- invalid table/function/channel names are rejected before SQL execution;
- broadcast rows are claimed with `SKIP LOCKED` and never processed by two
  nodes;
- rows for different streams can be processed concurrently;
- later same-stream rows are not claimed while an older same-stream row is
  unfinished;
- final failures keep inspection data;
- `postgres_exhausted_broadcast_policy=skip` lets later same-stream rows move
  after an exhausted row;
- `postgres_exhausted_broadcast_policy=block` keeps later same-stream rows
  blocked behind an exhausted row;
- successful broadcast rows are acked;
- notification payloads include the changed stream name;
- nodes ignore notifications for unsubscribed streams;
- repeated wakeups or fallback ticks do not duplicate delivery;
- two nodes subscribed to the same stream both receive the same fanout row;
- one node receives same-stream rows in insertion order;
- cursors advance independently per stream after delivery.

Batching and stress cases:

- coalesce multiple changed subscribed streams into one catch-up pass;
- verify one catch-up query can fetch rows for more than one stream;
- verify fallback polling batches subscribed stream cursors instead of issuing
  one query per stream;
- enforce a per-stream row limit so one hot stream does not starve other
  changed streams;
- include unsubscribed streams in notification payloads and assert they are not
  included in the batch query;
- subscribe to many streams, publish to a small subset, and assert poller query
  work is bounded by the changed subset, not every subscribed stream;
- publish bursts to many streams at once and verify wake-up coalescing still
  drains all rows;
- run concurrent publishers for the same stream and assert delivered order is
  still stream ordered;
- simulate a slow or failing consumer for one stream and verify unrelated
  streams continue to move;
- temporarily disable notifications and verify fallback polling catches up
  without duplicates.

Targeted commands:

```sh
go test ./postgres -run Postgres
go test ./broadcast -run Postgres
go test ./pubsub -run Postgres
go test ./config ./cli
go test ./... -run Postgres
```

Run broader `go test ./...` once the targeted suites pass and local Redis/NATS
dependencies are available or isolated.
