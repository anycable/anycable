# Guidance for running Benchi benchmarks

Loaded as context when an assistant works inside `etc/benchi/`. This file is
for **operating** the benchmarks; for modifying Benchi itself, see
`README.md` and the implementation plan in `docs/plans/`.

## Always pass `--non-interactive`

When you run a scenario binary on the user's behalf, pass `--non-interactive`.
Without it, Benchi auto-detects a TTY and emits lifecycle logs plus a
progress bar on stderr — useful for a human, noisy for you. The flag
suppresses everything except the key=value summary on stdout, which is what
you want to parse and report back.

```sh
go run ./scenarios/stress_publications \
  -c 1000 -r 100 -d 10s -S 10 -s 2 --non-interactive
```

Running from outside `etc/benchi/`? Either `cd` first (Benchi is a nested
Go module) or use `go run -C etc/benchi ./scenarios/...`.

## Required flags

The scenario refuses to run with zero defaults — there is no "safe accidental
benchmark." If you omit any of `-c`, `-r`, `-d`, `-S`, `-s`, the binary
exits non-zero with a usage message on stderr. This is intentional;
do not work around it.

| Flag | Meaning |
|---|---|
| `-c` | clients (WebSocket connections) |
| `-r` | publishes per second |
| `-d` | publish window duration (Go duration syntax: `1s`, `10s`, `500ms`) |
| `-S` | size of the stream universe (streams named `"1"`..`"S"`) |
| `-s` | per-client subscription count (must be ≤ `-S`) |

Optional knobs with safe defaults: `--drain-timeout=30s`, `--max-inflight=4096`,
`--publish-workers=64`, `--publish-batch=64`, `--setup-failure-tolerance=0`,
`--seed=1`.

`--publish-workers` is the size of the HTTP worker pool draining the
publisher queue. The default (64) is sized for the **embedded-server
scenario** where each POST to `/broadcast` triggers synchronous fan-out
across every subscribed client — high publisher concurrency multiplies
server-side work and can exhaust the Go OS-thread limit at high `-c`
(`runtime: failed to create new OS thread`). When pointing the publisher
at an **external broadcaster** (NATS, Redis, an edge service) each POST
is cheap and values up to ~1024 are reasonable; raise it to find that
broadcaster's ceiling.

`--publish-batch` is the maximum number of broadcasts a single worker
will coalesce into one POST under backpressure (`[{stream,data}, …]`
array body, supported by the AnyCable broadcast endpoint). At low rates
batches collapse to size 1 with no added latency; when the broadcast
endpoint can't keep up, batching amortizes per-request overhead and is
often the difference between a useful number and a 200× under-report.

## Picking a configuration

Average per-publish fan-out is `c × s / S`. Total expected throughput is
roughly `r × c × s / S` messages per second. Standard ladder, lowest to
highest load:

| Profile | Command | Target msgs/sec |
|---|---|---|
| Smoke (sanity check) | `-c 10 -r 10 -d 1s -S 1 -s 1` | 100 |
| Mid baseline | `-c 1000 -r 100 -d 10s -S 10 -s 2` | 20,000 |
| Realistic mixed | `-c 1000 -r 500 -d 10s -S 50 -s 10` | 100,000 |
| Full fan-out | `-c 10000 -r 100 -d 10s -S 1 -s 1` | 1,000,000 |

Pick the lowest profile that exercises the dimension the user cares about.
If they want hub regression detection, prefer "Realistic mixed" — every
dimension is non-zero so a change anywhere shows up.

## Reading the summary

The summary prints two families of rate metrics — throughput (messages
received by clients) and observed publish rate (POSTs the server
accepted) — each as an overall average plus four sliding-window
statistics (window size = `-d`, sampled every 100ms). The fast triage:

| Key | What it means | What to flag |
|---|---|---|
| `throughput_msgs_per_sec` | **Overall average:** total messages received divided by total receive-timeline wall seconds (publish + drain). | A whole-run average — useful as one-line summary, but `max` is the better success metric since a long drain tail can drag this number down. |
| `throughput_max_msgs_per_sec` | **Headline / success metric.** Highest receive rate sustained over any sliding `-d`-second window. | Compare to `expected_msgs_per_sec`. Above ~95% = clean run. Below ~80% = the system never managed the target rate even momentarily — real fan-out regression. |
| `throughput_min_msgs_per_sec` | Lowest receive rate over any full `-d`-second window. | A very low min means a stall — useful when investigating GC pauses, broker reconnections, or scheduler hiccups. |
| `throughput_p50_msgs_per_sec` | Median windowed rate. The "typical sustained" rate across the receive timeline. | If `p50` ≪ `max`, the peak was brief; the run is bursty rather than steady. Investigate with `min` and `p95`. |
| `throughput_p95_msgs_per_sec` | 95th-percentile windowed rate. The "near-peak" rate the system held most of the time. | If `max ≈ p95` the system was steady at peak; if `max ≫ p95` the peak was a single window. |
| `expected_msgs_per_sec` | Theoretical ceiling: `r × c × s / S`. | Constant for a given config — printed so `max / expected` is one division away. |
| `clients_complete` / `clients_short` | Reconciliation: did every client receive its share? | Any `clients_short > 0` is a **validity** failure. Report it; don't gloss over it. |
| `messages_missing` | How short the short clients were, summed. | Treat as binary: > 0 = invalid run. |
| `publications_issued` | Schedule cardinality: `floor(d / interval)`. Deterministic for `(r, d)`. | Should equal `r × d.Seconds()` for integer-second windows. This is the *intent*, not the *outcome*. |
| `publications_completed` | Broadcasts whose HTTP POST returned 2xx. The actual count the server accepted. | `completed < issued` means non-2xx responses or transport errors mid-flight — investigate the server log. |
| `publications_dropped` | Publisher queue was full (scheduler-fell-behind). | Any `> 0` means `--max-inflight` too small or HTTP path back-pressured. Recommend raising the cap or lowering `-r`. |
| `scheduler_late_publishes` | Diagnostic: catch-up fires (OS couldn't hit the tick on time). | Ratio over `publications_issued`: under 10% is healthy. Over 30% means the host is too loaded for stable rate measurement; suggest pinning `GOMAXPROCS` or lowering `-r`. |
| `observed_publish_rate` | `publications_completed / publish_window_wall_seconds` — the *real* overall rate at which the server accepted POSTs. | If this is well below `-r`, the broadcast endpoint is the bottleneck, **not** the WebSocket fan-out. Raising `--publish-batch` or adjusting the server is the lever, not raising `-r`. |
| `observed_publish_rate_max` / `_min` / `_p50` / `_p95` | Same sliding-window stats applied to the publisher's CompletedCount timeline (window = `-d`). | `max ≪ -r`: broadcast endpoint can't keep up even at its best window. `p50 ≪ max`: bursty completions, likely batching against backpressure. |
| `publish_window_wall_seconds` | Time from first enqueue to last completed POST. | If this is much larger than `-d`, the broadcast endpoint couldn't accept POSTs at the scheduled rate. Investigate before trusting throughput. |

**Why `max` (windowed) and not the overall average?** Because under
backpressure the publish window stretches well beyond `-d` (see
`publish_window_wall_seconds`), and the receive timeline keeps growing
for the duration of the drain. The overall average divides by a wall
clock that includes both the burst *and* the long-tail drain, so it
tells you nothing about whether the system ever achieved the target
rate. The sliding-window max answers the question that actually
matters: "did the system ever sustain the target rate over a window
the size of one publish duration?" `throughput_msgs_per_sec` is
reported alongside as a single-number whole-run figure but is **not**
the success metric.

Exit codes: `0` clean, `1` `clients_short > 0` or `publications_dropped > 0`,
`2` flag-parse error.

## When the user asks "is this a regression?"

A single run is not enough signal. Recommend the variance workflow:

```sh
rm -f baseline.txt
for i in 1 2 3 4 5; do
  go run ./scenarios/stress_publications \
    -c 1000 -r 100 -d 10s -S 10 -s 2 --seed $i --non-interactive \
  >> baseline.txt
done
awk -F= '/^throughput_max_msgs_per_sec/ { s+=$2; n++; if($2<min||n==1)min=$2; if($2>max)max=$2 } END { printf "n=%d mean=%.0f min=%.0f max=%.0f spread=%.1f%%\n", n, s/n, min, max, 100*(max-min)/(s/n) }' baseline.txt
```

If `spread` across 5 runs exceeds 5%, the host is too noisy for a clean
A/B — surface that to the user before they conclude anything. Real
regressions show as a consistent shift between two 5-run sets, not a
single-run delta.

## Failure modes to recognize

- **Pool setup error on stdout, exit code 1 immediately**: server fd limit,
  port exhaustion, or `--setup-failure-tolerance` too low. Re-run with
  `--setup-failure-tolerance 50` at high `-c`.
- **`publications_dropped > 0`**: scheduler outran the worker pool. Lower
  `-r`, raise `--max-inflight`, or accept the cap (the harness is doing
  what AE1 says it should — counting drops, not retrying).
- **`scheduler_late_publishes` very high (>50% of issued)**: the host
  scheduler can't hit sub-ms ticks reliably. Output is still **correct**
  (issued count is deterministic by design), but the publish distribution
  inside the window degenerated into catch-up bursts. Note this in the
  report.
- **`publish_window_wall_seconds` ≫ `-d` and `observed_publish_rate` ≪ `-r`**:
  the broadcast endpoint, not the WebSocket fan-out, is the bottleneck.
  The publisher kept enqueueing at `-r` but workers couldn't POST that
  fast. Don't read `throughput_max_msgs_per_sec` as a fan-out ceiling in
  this state — it's a broadcast-endpoint ceiling. Raising `--publish-batch`
  amortizes HTTP overhead and is often the right next move.
- **`observed_publish_rate_max` ≪ `-r` even though the publisher had
  budget**: the broadcast endpoint never sustained the requested rate
  over *any* `-d`-second window. Combined with low `observed_publish_rate_p50`,
  this points at a steady ceiling on the broadcast endpoint — not a
  one-off hiccup. Profile the receive-side fan-out or move the
  broadcaster (NATS/Redis/edge) out of process.
- **`throughput_max_msgs_per_sec` ≪ `expected_msgs_per_sec` with
  `clients_short=0` and `observed_publish_rate ≈ -r`**: the publisher
  delivered, the clients received, but the WebSocket fan-out path could
  never sustain the target rate over any `-d`-second window. This *is*
  the fan-out regression signal — investigate the hub/broker path before
  the publisher.
- **Throughput off by 5–15% with otherwise clean numbers**: in-process
  colocation noise. Run the variance workflow above to size it before
  drawing conclusions.

## What not to do

- Do **not** edit Benchi source to make a benchmark pass. The numbers are
  the message. If a result looks wrong, investigate; do not silence.
- Do **not** run `go test ./scenarios/stress_publications` to "verify the
  benchmark" — those are unit tests of the scenario binary, not
  benchmarks. Run the scenario itself.
- Do **not** invent flag defaults. The required flags are required by
  design; surface the usage to the user rather than picking values for
  them.
