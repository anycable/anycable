# Benchi

Benchi is an in-process benchmark harness for AnyCable. A scenario binary
embeds the full AnyCable server (`cli.NewRunner` → `Embedded`), spins up N
real WebSocket clients against a loopback `httptest.Server`, drives publishes
through the HTTP broadcast endpoint, and reconciles received-vs-expected
counts at the end of the publish window.

Benchi is not a replacement for `etc/k6/`. k6 exercises the deployed shape
(real network, real auth, real RPC). Benchi exercises the *in-process*
shape: same server code, same broker, same WebSocket framing, same HTTP
broadcast ingestion — but no network, no RPC, no auth. That makes Benchi
sensitive to small hub/broker/broadcaster deltas that k6 buries under
network noise, at the cost of being non-representative of production.

To keep the harness minimal, Benchi runs the server in **public-streams +
skip-auth mode**: `c.Streams.Public = true`, `c.SkipAuth = true`,
`c.RPC.Implementation = "none"`, channel `$pubsub`, plain stream names.
This is a *benchmark choice*, not a production template. See
[`docs/signed_streams.md`](../../docs/signed_streams.md) for the production
public-streams flow with stream signing.

---

## Running `stress_publications`

The v1 scenario dials `-c` clients, subscribes each to `-s` random streams
out of `[1..S]`, publishes at `-r` per second for `-d`, drains in-flight
messages, and prints a key=value summary on stdout.

```
cd etc/benchi
go run ./scenarios/stress_publications -c 1000 -r 100 -d 10s -S 100 -s 10
```

Expected output (numbers vary run-to-run):

```
throughput_msgs_per_sec=9876.50
clients_complete=1000
clients_short=0
messages_missing=0
publications_issued=1000
publications_dropped=0
observed_publish_rate=100.00
```

A run is **clean** when `clients_short=0` and `publications_dropped=0`.
The process exits non-zero if either is non-zero — so it composes with
shell pipelines and CI without parsing stdout.

Setup time (dial + subscribe + confirm for `-c × -s` channels) is not
counted in `-d`. Plan on a few seconds of setup before publishing starts at
`-c` in the thousands.

---

## Flag reference

All of `-c`, `-r`, `-d`, `-S`, `-s` are **required** — there are no
defaults. Running with no flags prints usage to stderr and exits non-zero,
so a stray invocation never silently runs a benchmark. The recommended
baseline that the planning team had in mind (`-c=100 -r=100 -d=10s -S=10
-s=1`) is a sensible starting point if you have no priors.

| Flag | Type | Default | Effect |
|---|---|---|---|
| `-c` | int | — (required, >0) | Number of WebSocket clients to dial. |
| `-r` | int | — (required, >0) | Publish rate (messages per second). The scheduler ticks at `1s / r`. |
| `-d` | duration | — (required, >0) | Publish window length. Setup and drain are excluded; throughput uses `-d` as denominator. |
| `-S` | int | — (required, >0) | Size of the stream universe; streams are named `"1"` through `"S"`. |
| `-s` | int | — (required, `0 < s <= S`) | Per-client subscription count (subset size). Each client subscribes to a random S-of-bigS subset, without replacement. |
| `--drain-timeout` | duration | `30s` | Max wall time to wait for in-flight messages after the publish window closes. Hitting it marks clients short, doesn't hang. |
| `--max-inflight` | int | `4096` | Publisher work-queue depth. The scheduler is non-blocking on enqueue; if the queue is full, the publication slot is dropped (counted in `publications_dropped`). |
| `--setup-failure-tolerance` | int | `0` | Max client setup failures (build / connect / subscribe) before aborting. At `-c` in the thousands, raising this absorbs transient subscribe-confirm noise. |
| `--seed` | int64 | `1` | RNG seed for stream-subset selection and per-tick stream choice. Same seed, same subsets, same publish sequence. |

---

## Reported metrics

Every summary line is `key=value\n`. Keys are stable; values are plain
text (floats fixed at 2dp, integers unformatted) so `awk`/`cut`/`benchstat`
can consume them without a parser.

| Key | Definition |
|---|---|
| `throughput_msgs_per_sec` | `total_received / d.Seconds()`. Denominator is the publish window only — drain time is excluded. |
| `clients_complete` | Number of clients whose received count met or exceeded their expected count. |
| `clients_short` | Number of clients whose received count was below their expected count. |
| `messages_missing` | `sum of max(0, expected_i - received_i)` across short clients. **Conflates two failure modes**: server-side delivery failure (broker or broadcaster) and client-side miss (cap-16 per-subscription channel filled before the read loop caught up). Either is a benchmark-invalidating signal; in v1 the single number is enough to *trigger* an investigation, but the operator must look at both sides before drawing a conclusion. |
| `publications_issued` | Total publishes the scheduler successfully handed to a worker. |
| `publications_dropped` | Publishes the scheduler tried to enqueue while the worker queue was full. Non-zero means `--max-inflight` is too low for `-r` × per-call HTTP latency, *or* the embedded server is back-pressuring HTTP broadcasts (which itself is a signal). |
| `observed_publish_rate` | `publications_issued / d.Seconds()`. Should track `-r` closely; a large gap means the scheduler couldn't keep up. |

### Quick interpretation guide

| Symptom | Look here first |
|---|---|
| `clients_short > 0` and `messages_missing` is small | Most likely client-side: cap-16 per-subscription channel briefly overflowed. Reduce `-r` or `-s`, or investigate fork's drain rate. |
| `clients_short > 0` and `messages_missing` is large | Most likely server-side: broker or broadcaster failed to deliver. Re-run with the embedded server's logger captured (`ServerConfig.Logger`) to see broker errors. |
| `publications_dropped > 0` | Scheduler-fell-behind. Either lower `-r`, raise `--max-inflight`, or accept the cap; the publisher is intentionally non-blocking, not retried. |
| `observed_publish_rate << -r` | Same cause: the scheduler's enqueue path is saturating. |
| Setup hangs at high `-c` | Subscribe-confirm round-trip storm. Try `--setup-failure-tolerance 50` to keep a partial pool, or stage the run at lower `-c` to confirm the fork is the bottleneck. |

---

## How to add a new scenario

Each scenario is its own `main` package under `scenarios/<name>/`. Copy
`scenarios/stress_publications/` as a starting point — it already wires
the four primitives together end-to-end.

The library contract is the public surface of `etc/benchi/lib/`:

- `lib.BuildServer(lib.ServerConfig{...})` — embedded server. The
  `ServerConfig` struct is the **scenario-config contract**: today it
  exposes `Logger`, `BrokerAdapter`, and `ExtraOptions`. Future scenarios
  flip those fields rather than reaching into `cli.NewConfig` directly.
- `lib.BuildPool(lib.PoolConfig{...})` — N connected clients with bounded
  parallelism and per-client stream subsets.
- `lib.NewPublisher(url, maxInflight, opts...)` — bounded async publisher
  with `Publish` / `IssuedCount` / `DroppedCount` / `CompletedCount` /
  `TargetCounts` / `Close`. Workers coalesce queued tasks into batched
  POSTs under backpressure. Options: `lib.WithWorkers(n)` (default 64,
  sized for the embedded server; raise for external broadcasters where
  each POST is cheap), `lib.WithBatchSize(n)` (default 64; 1 disables
  batching).
- `lib.NewAccumulator()` — per-client receive counter; pass to
  `PoolConfig.Accumulator` so the drain goroutines feed it for free.

Switch broker via `ServerConfig.BrokerAdapter` (`""` = LegacyBroker /
zero history, `"memory"` = in-process history, `"nats"` = external).
Append broadcasters or swap the subscriber via `ServerConfig.ExtraOptions`.

Only one `Server` per process is supported in v1: `cli.NewRunner` mutates
package-level globals (`server.Host`, `server.MaxConn`, `server.Logger`,
`server.SSL`), so two concurrent `BuildServer` calls would race. Always
`Shutdown` the previous server before building another.

---

## Variance and repeat runs

A single Benchi run is one sample from a noisy distribution. In-process
colocation (clients + publisher + server share one Go scheduler, one
heap, one GC) is sensitive to background load, CPU frequency scaling,
and GC pacing. Single-shot numbers are useful for spotting order-of-
magnitude regressions; everything finer needs repeats.

The recommended workflow is N independent runs (start with 5, raise to 10
if results look bimodal) followed by `benchstat`-style aggregation:

```sh
for i in 1 2 3 4 5; do
  go run ./scenarios/stress_publications \
    -c 1000 -r 100 -d 10s -S 100 -s 10 --seed $i \
  >> baseline.txt
done

awk -F= '/^throughput_msgs_per_sec/ { s+=$2; n++ } END { print s/n }' baseline.txt
```

For A/B comparison (baseline branch vs. change branch), capture two
files of N runs each and feed them to your favorite stats tool (`benchstat
baseline.txt change.txt`, R, a notebook, etc.). v1 ships single-shot
numbers; in-app variance and warmup are intentionally deferred.

Two reproducibility levers:

- **Pin `GOMAXPROCS`** to match the comparison environment (e.g.,
  `GOMAXPROCS=4 go run ...`). Otherwise auto-detected CPU count changes
  the noise floor between machines.
- **Vary `--seed`** across runs (or fix it for paired comparisons). The
  seed controls both stream-subset selection and the per-tick stream
  picked by the publisher.

---

## Troubleshooting

- **"`pool setup failed: N failure(s) exceed tolerance 0`"**
  At least one client failed to build / connect / subscribe. The error
  lists `client N: <phase>: <err>` for each failure. At low `-c` this
  usually means a misconfigured server (check `WebSocketURL` reachability
  in your tweak). At high `-c` it usually means a transient subscribe-
  confirm timeout — raise `--setup-failure-tolerance` to absorb a few
  drops without losing the run.

- **All clients short, zero throughput**
  Most likely an async server failure that surfaces as a symptom rather
  than an error. `cli.Embedded` does not expose its private `errChan` in
  v1, so the only signal is shape-side. Re-run with
  `ServerConfig.Logger` set to a real logger (e.g.,
  `slog.New(slog.NewTextHandler(os.Stderr, nil))`) to see broker or
  broadcaster start errors.

- **Server log lines bleed into the summary on stdout**
  `BuildServer` discards server logs by default. If you set a custom
  `ServerConfig.Logger`, point it at stderr or a file — Benchi's stdout
  contract is summary-only (R13), and downstream `awk`/`benchstat`
  pipelines will choke on log lines.

- **`-c` in the low thousands feels slow or hangs**
  The forked cinemast client spawns goroutines per subscription. At
  `-c 1000 -s 10` that's 10,000 receive goroutines plus 10,000 cap-16
  channels — workable, but not free. Run the lib-level smoke (`go test
  ./lib/ -run TestBuildPool_AllSubscribeOk -count=1`) in isolation to
  confirm the fork keeps up; if it does, the bottleneck is elsewhere
  (scheduler at `-r`, broker, broadcaster). If the fork itself is slow,
  it's ours — trim per-connection goroutines or switch to a single
  demuxer in `etc/benchi/client/`.

- **`publications_dropped > 0` with plenty of `--max-inflight` headroom**
  HTTP broadcasts are stalling at the server side faster than the worker
  pool can clear. Capture the embedded server's logger and look for
  broadcaster errors; if there are none, the bottleneck is the broker or
  the hub fan-out — exactly the kind of regression Benchi is built to
  surface.
