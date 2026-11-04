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

## Running `reconnect_floods`

`reconnect_floods` reproduces reconnect storms against an RPC-backed
server. It is the only scenario that uses RPC: the embedded server calls a
fake HTTP RPC application (`lib.RPCServer`) with configurable latency.

The run has three steps:

1. **Setup.** `-c` clients connect at `--ramp-rate` per second. Each client
   subscribes to `-s` RPC-backed channels. Setup ends when all clients are
   healthy (all subscriptions confirmed).
2. **Waves.** After `--settle`, each wave drops N random open connections
   at once, without a close frame (like an LB restart). Waves fire every
   `--wave-interval`.
3. **Recovery.** After the last wave, the run waits up to
   `--recovery-timeout` for all clients to become healthy again.
4. **Drain.** If the clients recover, the run then waits for the server
   disconnect queue to become empty. There is no timeout for this step:
   press Ctrl-C to stop.

Clients behave like browser clients:

- No ping for `--ping-timeout` (default 6s) → reconnect. The watch starts
  when the socket opens, not after the welcome message.
- A subscribe command not confirmed in `--sub-retry-interval` (default 5s)
  → send it again, up to `--sub-retries-max` times (default 2). When the
  last one is not confirmed in time → reconnect. With the defaults, a
  client reconnects 15s after the first subscribe command.
  `--sub-retries-max 0` reconnects after the first interval.
- Subscribe commands go out all at once after the welcome message. With
  `--sequential-subscribe`, the client sends the next command only after
  the previous one is confirmed. Each retry interval then starts at its own
  command.

`--probes` extra clients (default 10) measure how fast the server handles
client messages. A probe has no subscriptions, and waves do not drop it.
Every `--probe-interval` (default 1s), it sends a `ping` command and
measures the time to the `pong` reply. The server answers pings without
RPC calls. A slow pong therefore means that the server reads or handles
client messages late. A busy RPC queue alone does not slow it down.
- Reconnects use exponential backoff: `--backoff-min` doubles per failed
  attempt up to `--backoff-max`, with ±50% jitter.

Example of a scaled-down storm (see "Scaling" below):

```
cd etc/benchi
go run ./scenarios/reconnect_floods -c 3000 --waves 1000,1000,3000 \
  --wave-interval 40s --rpc-latency 100ms \
  --disconnect-rate 10 --disconnect-backlog 256
```

In a terminal, the scenario shows a live dashboard on stderr: client
states, RPC calls per second by method, the disconnect queue, goroutines
and heap, handshake, confirmation and probe timings, close reasons, and the
current command stall. Pass `--non-interactive` to get only the summary.

To stop a run early, press Ctrl-C (or send SIGTERM). The run stops
waiting, prints the summary with `interrupted=true` and writes the
timeline. A second Ctrl-C kills the process at once. This is useful when
the disconnect queue drains slowly: the disconnector makes one call at a
time.

| Exit code | Meaning |
|---|---|
| `0` | All clients healthy after the last wave. |
| `1` | Setup or recovery failed. |
| `2` | Flag error. |
| `130` | Interrupted. |

### Scaling

The production incident had tens of thousands of reconnects per wave.
Thousands of clients show the same dynamics when the ratios stay the
same. Experiments with `-c 3000`, `-s 3`, 100ms RPC latency show one ratio
that decides the outcome:

- **Reconnect demand vs. RPC capacity.** Each reconnect costs
  1 + `-s` RPC calls. Capacity is `--rpc-concurrency` ÷ RPC latency
  (28 ÷ 100ms ≈ 280 calls/s), shared by Connect, Command and Disconnect.
  When reconnecting clients × (1 + `-s`) ÷ backoff exceeds capacity,
  the slot wait grows past the subscribe retries. Clients then time out,
  reconnect, and keep the storm alive.

Measured before subscribe retries and the disconnect queue limit existed:
impatient clients (`--sub-retries-max 0`) and a blocking disconnect queue
of 4096. Single wave, default disconnect settings:

| Wave | Change | Outcome |
|---|---|---|
| 500 | — | recovers in ~13s |
| 1000 | — | recovers in ~74s |
| 1500 | — | stuck: 1500 clients cycle forever |
| 1500 | `--rpc-wait-timeout 1s` | recovers in ~46–48s (same with `500ms`) |
| 2000 | — | stuck |
| 3000 | `--backoff-max 30s` | recovers in ~160–210s |
| 3000 | `--rpc-concurrency 112` | recovers in ~50s |

`--disconnect-rate` and `--disconnect-backlog` did not change recovery
time in these runs. During a storm the disconnector gets almost no RPC
slots, and the queue stays full. With the blocking queue, every closed
session then blocked a goroutine in `DisconnectQueue.Enqueue`: this was
the memory growth. Now the queue discards Disconnect calls over
`--disconnect-backlog` instead.

The disconnect queue drains long after the clients recover. The
disconnector makes one call at a time, so its real rate is
min(`--disconnect-rate`, 1 ÷ Disconnect latency). With 100ms latency that
is 10 calls/s:

| Wave | Clients healthy | Disconnect queue empty | Peak queue |
|---|---|---|---|
| 500 | ~13s | ~101s | 856 |
| 1500, `--rpc-wait-timeout 1s` | ~48s | >5 min | 3407 |
| 3000, `--backoff-max 30s` | ~158s | >5 min (still full) | 4096 |

During the drain, Disconnect calls run at a flat rate and Command calls
are 0. Use `--rpc-disconnect-latency` to change the drain rate.

### Presets

Use these three runs as starting points. All use `-c 3000`, `-s 3`, 100ms
RPC latency and 28 RPC slots. `--ramp-rate 40` keeps the setup itself from
starting a storm; setup takes about 75s. `--rpc-disconnect-latency 10ms`
lets the disconnect queue drain at `--disconnect-rate` (100/s), like in
production.

**Healthy.** The server absorbs the wave. All clients recover in ~8s; the
disconnect queue (peak ~480) is empty after ~13s. Some subscribe commands
are re-sent, but the server answers them without RPC calls.

```
go run ./scenarios/reconnect_floods -c 3000 --waves 500 \
  --rpc-latency 100ms --rpc-disconnect-latency 10ms \
  --ramp-rate 40 --setup-timeout 3m
```

**Slow recovery.** All clients reconnect at once. The server stays
saturated and recovers in ~95s (~60s with `--sequential-subscribe`). Most
failed attempts are Connect calls that wait longer than
`--rpc-wait-timeout`: the server closes these connections
(`close_server_closed`).

```
go run ./scenarios/reconnect_floods -c 3000 --waves 3000 --backoff-max 30s \
  --rpc-latency 100ms --rpc-disconnect-latency 10ms \
  --ramp-rate 40 --setup-timeout 3m --recovery-timeout 5m
```

**Never recovers (impatient clients).** Clients reconnect when a
subscription is not confirmed in 5s, with no retries. About 1500 clients
cycle through welcome, subscribe timeout and reconnect. The RPC queue
stays at ~780 calls, so each call waits ~2.7s. A client needs four calls
one after another (Connect and three subscribes), so no client becomes
healthy. The disconnect queue stays at its limit. The run ends with
`recovered=false` and exit code `1`. With the default retries, the same
wave recovers in ~23s.

```
go run ./scenarios/reconnect_floods -c 3000 --waves 1500 \
  --rpc-latency 100ms --rpc-disconnect-latency 10ms --sub-retries-max 0 \
  --ramp-rate 40 --setup-timeout 3m --recovery-timeout 5m
```

To move the threshold, change `--backoff-max`, `--rpc-concurrency` or
`--rpc-latency`. Add `--timeline run.csv` for a per-second CSV.

### `reconnect_floods` flags

| Flag | Default | Effect |
|---|---|---|
| `-c` | — (required) | Number of clients. |
| `--waves` | — (required) | Comma-separated wave sizes, e.g. `1000,1000,3000`. Each size must be ≤ `-c`. |
| `-s` | `3` | RPC-backed subscriptions per client. |
| `--wave-interval` | `60s` | Time between waves. |
| `--ramp-rate` | `200` | Initial connects per second during setup. |
| `--setup-timeout` | `2m` | Max time for all clients to become healthy before the first wave. |
| `--settle` | `5s` | Steady-state time between setup and the first wave. |
| `--recovery-timeout` | `5m` | Max time after the last wave for all clients to become healthy. The drain step after it has no timeout. |
| `--rpc-latency` | `100ms` | Default latency of every fake RPC call. |
| `--rpc-jitter` | `0.3` | Each call takes latency × (1 ± jitter), uniform. |
| `--rpc-connect-latency`, `--rpc-command-latency`, `--rpc-disconnect-latency` | `0` | Per-method latency; `0` uses `--rpc-latency`. |
| `--rpc-concurrency` | `28` | Server `rpc_concurrency`. |
| `--rpc-wait-timeout` | command timeout | Server `rpc_wait_timeout`: max wait for a free RPC slot. A negative value (the default) means the command timeout (3s); `0` means no limit. |
| `--disconnect-rate` | `100` | Server `disconnect_rate` (calls per second). |
| `--disconnect-backlog` | `8192` | Server `max_disconnect_backlog_size`: Disconnect calls over the limit are discarded. |
| `--ping-interval` | `3` | Server `ping_interval`, in seconds. |
| `--ping-timeout` | `6s` | Client ping watchdog. |
| `--sub-retry-interval` | `5s` | Wait for a confirmation before the client sends the subscribe command again. |
| `--sub-retries-max` | `2` | Re-sends per channel. The client reconnects when the last one is not confirmed in `--sub-retry-interval`. |
| `--sequential-subscribe` | `false` | Send the next subscribe command only after the previous one is confirmed. |
| `--backoff-min` / `--backoff-max` | `500ms` / `10s` | Client reconnect backoff. |
| `--probes` | `10` | Probe clients that measure the ping → pong time. `0` turns probes off. |
| `--probe-interval` | `1s` | Time between probe pings. A probe sends the next ping only after the pong. |
| `--seed` | `1` | Seed for wave selection and backoff jitter. |
| `--pprof` | — | Serve `net/http/pprof` on this address, e.g. `localhost:6060`. The server and the clients share the process, so profiles include both. |
| `--timeline` | — | Write a per-second CSV (client states, RPC calls/s by method, server gauges, close reasons, timings) to this path. Use it to see *when* commands stall, not only *how long*. |

### `reconnect_floods` summary keys

| Key | Definition |
|---|---|
| `interrupted` | `true` when the run was stopped with Ctrl-C or SIGTERM. Waves after the interrupt do not fire. |
| `setup_ok`, `recovered` | `setup_ok`: all clients were healthy before the first wave. `recovered`: all clients were healthy after the last wave. |
| `wave_N_dropped` | Connections the wave closed. Can be lower than `wave_N_size` during a storm: clients in backoff have no connection to drop. |
| `wave_N_recovery_seconds` | Time from the wave to all clients healthy. `-1` = not recovered before the next wave (or the recovery timeout). |
| `wave_N_drained_seconds` | Time from the wave to an empty disconnect queue, after recovery. `-1` = not drained before the next wave (or the interrupt). |
| `connection_attempts` | All connection attempts, setup included. |
| `reconnects` | Connection attempts after each client's first one: `connection_attempts` minus the first connects. |
| `subscribe_retries` | Subscribe commands sent again because no confirmation arrived in `--sub-retry-interval`. |
| `close_<reason>` | Attempts that ended with this reason: `ping_timeout`, `subscribe_timeout`, `dropped` (by a wave), `server_closed`, `server_disconnect`, `rejected`, `dial_error`. |
| `rpc_<method>_calls` | Calls the fake RPC completed, by method. Disconnects at shutdown are included. |
| `rpc_canceled_calls` | Server counter `rpc_canceled_total`: calls that waited for an RPC slot and were dropped because the session had closed meanwhile. They never reach the RPC server. |
| `rpc_wait_timeout_calls` | Server counter `rpc_wait_timeout_total`: calls that waited for an RPC slot longer than `--rpc-wait-timeout`. They never reach the RPC server. |
| `rpc_connect_after_close_calls`, `rpc_command_after_close_calls` | Connect and Command calls that reached the fake RPC after the client had closed the connection. The server did this work for nothing. Each connection attempt passes its ID in the `bench_sid` URL parameter. The fake RPC uses it as the connection identifier. |
| `disconnect_discarded` | Server counter `disconnect_queue_discarded_total`: closed sessions that the full disconnect queue dropped. These sessions get no Disconnect call. |
| `max_command_stall_seconds` | Longest run of seconds in which clients waited for confirmations but no Command call completed. |
| `welcome_*_ms` | Dial start → welcome message. |
| `confirm_*_ms` | First subscribe command for a channel → its confirmation (retries included). |
| `healthy_*_ms` | Dial start → last confirmation. |
| `probe_*_ms` | Probe `ping` command → `pong` reply. |
| `peak_disconnect_queue`, `peak_rpc_pending`, `peak_server_sessions` | Peaks of the server gauges `disconnect_queue_size`, `rpc_pending_num`, `clients_num`. |
| `peak_goroutines`, `peak_heap_mb` | Process-wide peaks. Clients run in the same process, so these include client memory. |

---

## How to add a new scenario

Each scenario is its own `main` package under `scenarios/<name>/`. Copy
`scenarios/stress_publications/` as a starting point — it already wires
the four primitives together end-to-end.

The library contract is the public surface of `etc/benchi/lib/`:

- `lib.BuildServer(lib.ServerConfig{...})` — embedded server. The
  `ServerConfig` struct is the **scenario-config contract**: today it
  exposes `Logger`, `BrokerAdapter`, `ExtraOptions`, `RPCHost` (switch to
  HTTP RPC mode) and `Configure` (tune `config.Config` before the runner
  starts). Future scenarios
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
