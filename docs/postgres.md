# Postgres signalling

AnyCable-Go can use PostgreSQL as a signalling backend for broadcasts and
node-to-node pub/sub. This is useful when an application already operates
Postgres and wants to avoid a separate Redis or NATS dependency for AnyCable
signalling.

Postgres signalling stores message payloads in tables. PostgreSQL
`LISTEN/NOTIFY` is used only as a wake-up mechanism, so delivery does not depend
on fitting the full message into a notification payload.

![Postgres signalling data model](./images/postgres-signalling-data-model.png)

## Usage

Enable the Postgres broadcaster and pub/sub adapter together for multi-node
deployments:

```sh
anycable-go \
  --postgres_url=postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable \
  --broadcast_adapter=postgres \
  --pubsub=postgres
```

When `--broadcast_adapter=postgres` is used and `--pubsub` is not set
explicitly, AnyCable-Go selects `pubsub=postgres` so table-backed broadcasts can
reach every node in a cluster.

## Schema

AnyCable-Go owns the required Postgres signalling schema by default. On startup,
it creates or actualizes the required objects and then validates the resulting
shape before accepting traffic.

The default schema contains:

- `anycable_stream_offsets`: latest allocated offset for each signalling scope
  and stream;
- `anycable_broadcasts`: app-to-AnyCable broadcast queue;
- `anycable_pubsub`: node-to-node pub/sub catch-up log;
- `anycable_publish(stream text, payload text, meta text default '{}')`;
- `anycable_remote_command(payload text, meta text default '{}')`;
- insert triggers that send small JSON wake-up notifications.

Set `--postgres_ensure_schema=false` only when another migration system manages
these objects. This disables DDL creation and modification, but startup still
validates the required tables, uniqueness contracts, triggers, and SQL
functions. A missing or incompatible externally managed schema fails startup.

## Publishing

Applications should publish through SQL functions instead of inserting rows
directly.

Use `anycable_publish` for normal broadcasts:

```sql
SELECT anycable_publish(
  'chat:1',
  '{"stream":"chat:1","data":"{\"text\":\"Hello\"}"}',
  '{}'
);
```

The `payload` argument is the AnyCable publication JSON accepted by broadcast
adapters. The `stream` argument is used by the Postgres queue for per-stream
offset allocation and ordering, so it should match the publication stream.

Use `anycable_remote_command` for internal remote commands:

```sql
SELECT anycable_remote_command(
  '{"type":"disconnect","identifier":"..."}',
  '{}'
);
```

Both functions return the allocated broadcast-scope offset. Payload and metadata
are stored as opaque `text`; SQL does not parse them as JSON.

## Delivery Model

![Postgres signalling dataflow](./images/postgres-signalling-dataflow.png)

The Postgres broadcaster is a single-consumer queue. AnyCable nodes claim rows
with `FOR UPDATE SKIP LOCKED`, process them through the broker/pub-sub path, and
delete successful rows.

The pub/sub adapter stores inter-node fanout rows in `anycable_pubsub`. Each
node tracks local per-stream cursors for active subscriptions and batch-fetches
new rows for changed streams. Periodic polling remains the correctness fallback
when notifications are missed, delayed, or coalesced.

Ordering is guaranteed per stream, not globally across unrelated streams. Rows
for different streams can proceed independently.

## Failures And Cleanup

Failed broadcast rows are retried until `postgres_max_attempts` is reached. A
terminally failed row keeps inspection metadata such as `claimed_by`,
`claimed_at`, `attempts`, `last_error`, and `exhausted_at`.

The `postgres_exhausted_broadcast_policy` setting controls later rows for the
same stream:

- `skip` lets later rows proceed after the older row records `exhausted_at`;
- `block` keeps later rows blocked until an operator removes or repairs the
  exhausted row.

The default is `skip`.

Cleanup removes old pub/sub rows and exhausted `skip` broadcast rows according
to `postgres_retention_ttl` and `postgres_cleanup_interval`. Stream offsets are
kept in `anycable_stream_offsets`, so cleanup does not reset per-stream offsets.

## Configuration

See [configuration](./configuration.md#postgres-configuration) for all Postgres
settings.
