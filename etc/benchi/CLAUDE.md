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
`--setup-failure-tolerance=0`, `--seed=1`.

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

Eight keys come back on stdout. The fast triage:

| Key | What it means | What to flag |
|---|---|---|
| `throughput_msgs_per_sec` | Primary metric: total received / d.Seconds(). | Should approximate `r × c × s / S` to within ~5% on a clean host. |
| `clients_complete` / `clients_short` | Reconciliation: did every client receive its share? | Any `clients_short > 0` is a **validity** failure. Report it; don't gloss over it. |
| `messages_missing` | How short the short clients were, summed. | Treat as binary: > 0 = invalid run. |
| `publications_issued` | Schedule cardinality: `floor(d / interval)`. Deterministic for `(r, d)`. | Should equal `r × d.Seconds()` for integer-second windows. |
| `publications_dropped` | Publisher queue was full (scheduler-fell-behind). | Any `> 0` means `--max-inflight` too small or HTTP path back-pressured. Recommend raising the cap or lowering `-r`. |
| `scheduler_late_publishes` | Diagnostic: catch-up fires (OS couldn't hit the tick on time). | Ratio over `publications_issued`: under 10% is healthy. Over 30% means the host is too loaded for stable rate measurement; suggest pinning `GOMAXPROCS` or lowering `-r`. |
| `observed_publish_rate` | `issued / d.Seconds()`. With absolute scheduling, ≈ `-r` by construction. | Drift from `-r` means `d` was inexact for the schedule (e.g., non-integer-second `-d`). Rare. |

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
awk -F= '/^throughput_msgs_per_sec/ { s+=$2; n++; if($2<min||n==1)min=$2; if($2>max)max=$2 } END { printf "n=%d mean=%.0f min=%.0f max=%.0f spread=%.1f%%\n", n, s/n, min, max, 100*(max-min)/(s/n) }' baseline.txt
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
