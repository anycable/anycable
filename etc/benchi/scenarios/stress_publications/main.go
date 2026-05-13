// stress_publications is the first benchi scenario: dial C clients,
// subscribe each to S random streams out of [1..bigS], publish at -r per
// second for -d, and report received-vs-expected reconciliation on stdout.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/anycable/anycable-go/etc/benchi/lib"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, runOpts{}))
}

// runConfig is the parsed shape of the CLI flags. Kept separate from runOpts
// so test-only seams live in their own struct and can't accidentally leak
// into the CLI.
type runConfig struct {
	c              int
	r              int
	d              time.Duration
	bigS           int
	s              int
	drainTimeout   time.Duration
	maxInflight    int
	publishWorkers int
	publishBatch   int
	tolerance      int
	seed           uint64
	nonInteractive bool
}

// runOpts is the testing seam. Production main constructs an empty value;
// tests fill in hooks to mutate the running scenario without exposing the
// hooks via CLI flags.
type runOpts struct {
	// streamSubsets, when non-nil, overrides the pool's random subset
	// generation. Used by SubscribeFailure tests to force rejection on a
	// specific client.
	streamSubsets [][]string

	// broadcastURL, when non-empty, replaces the publisher target. Used by
	// the Cadence test to point publishes at a deliberately slow proxy.
	broadcastURL string

	// setupHook, when non-nil, runs after the server, pool, and publisher
	// are built but before the publish window opens. Used by OneClientShort
	// and DrainTimeoutHonored tests to mutate pool state.
	setupHook func(server *lib.Server, pool *lib.ClientPool, pub *lib.Publisher)
}

// run is the testable entry point. Returns the process exit code. Writes
// the operator-facing summary to stdout; usage and setup errors go to
// stderr.
func run(args []string, stdout, stderr io.Writer, opts runOpts) int {
	cfg, err := parseFlags(args, stderr)
	if err != nil {
		// flag.Parse already wrote -h/usage to stderr; nothing to add here.
		if errors.Is(err, flag.ErrHelp) {
			return 2
		}
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}

	return runWithConfig(cfg, opts, stdout, stderr)
}

func parseFlags(args []string, stderr io.Writer) (runConfig, error) {
	fs := flag.NewFlagSet("stress_publications", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var cfg runConfig
	fs.IntVar(&cfg.c, "c", 0, "number of clients (required, must be > 0)")
	fs.IntVar(&cfg.r, "r", 0, "publish rate in messages per second (required, must be > 0)")
	fs.DurationVar(&cfg.d, "d", 0, "publish window duration (required, must be > 0)")
	fs.IntVar(&cfg.bigS, "S", 0, "size of the stream universe; streams are named 1..S (required, must be > 0)")
	fs.IntVar(&cfg.s, "s", 0, "per-client subscription count (required, 0 < s <= S)")
	fs.DurationVar(&cfg.drainTimeout, "drain-timeout", 30*time.Second, "max wall time to wait for in-flight messages after the publish window closes")
	fs.IntVar(&cfg.maxInflight, "max-inflight", 4096, "publisher queue depth — full queue increments publications_dropped")
	fs.IntVar(&cfg.publishWorkers, "publish-workers", 64, "HTTP worker goroutines draining the publisher queue (sized for embedded server; raise for external broadcasters where each POST is cheap)")
	fs.IntVar(&cfg.publishBatch, "publish-batch", 64, "max broadcasts coalesced into a single POST under backpressure (1 disables batching)")
	fs.IntVar(&cfg.tolerance, "setup-failure-tolerance", 0, "max client setup failures before aborting")
	var seed int64
	fs.Int64Var(&seed, "seed", 1, "RNG seed for stream-subset selection and per-tick stream choice")
	fs.BoolVar(&cfg.nonInteractive, "non-interactive", false, "suppress lifecycle logs and progress bar on stderr; emit only the key=value summary on stdout (auto-on when stderr is not a TTY)")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	cfg.seed = uint64(seed) //nolint:gosec // operator-supplied seed; sign reinterpretation is intentional

	if cfg.c <= 0 || cfg.r <= 0 || cfg.d <= 0 || cfg.bigS <= 0 || cfg.s <= 0 {
		fs.Usage()
		return cfg, errors.New("required flags missing: -c, -r, -d, -S, -s must all be > 0")
	}
	if cfg.s > cfg.bigS {
		return cfg, fmt.Errorf("-s (%d) must be <= -S (%d)", cfg.s, cfg.bigS)
	}
	return cfg, nil
}

// summary holds the reconciled numbers printed at the end of a run.
//
// Throughput is reported as the overall average plus four sliding-window
// statistics over the receive timeline (window = -d, sampled at 100ms):
// max (headline / success metric — highest sustained rate over any -d
// window), min, p50, p95. expectedMsgsPerSec is the theoretical ceiling
// `r × c × s / S` so the operator can read max as a fraction of expected.
//
// observedPublishRate mirrors the same structure over the publisher's
// CompletedCount timeline: overall (completed / publishWallSecs) plus the
// windowed max/min/p50/p95. If overall ≪ -r, the broadcast endpoint, not
// the WebSocket fan-out, is the bottleneck.
type summary struct {
	throughputMsgsPerSec    float64
	throughputMaxMsgsPerSec float64
	throughputMinMsgsPerSec float64
	throughputP50MsgsPerSec float64
	throughputP95MsgsPerSec float64
	expectedMsgsPerSec      float64
	clientsComplete         int
	clientsShort            int
	messagesMissing         int
	publicationsIssued      int64
	publicationsCompleted   int64
	publicationsDropped     int64
	schedulerLatePublishes  int
	observedPublishRate     float64
	observedPublishRateMax  float64
	observedPublishRateMin  float64
	observedPublishRateP50  float64
	observedPublishRateP95  float64
	publishWindowWallSecs   float64
}

func runWithConfig(cfg runConfig, opts runOpts, stdout, stderr io.Writer) int {
	ctx := context.Background()
	u := newUI(stderr, cfg.nonInteractive)

	u.logf("starting embedded AnyCable server...")
	server, err := lib.BuildServer(lib.ServerConfig{})
	if err != nil {
		fmt.Fprintln(stdout, "server build failed:", err)
		return 1
	}
	defer server.Shutdown(ctx) //nolint:errcheck // best-effort shutdown
	u.logf("server ready at %s", server.WebSocketURL())

	var connected, subscribed atomic.Int64
	acc := lib.NewAccumulator()
	poolStart := time.Now()
	u.logf("building pool: %d clients × %d subscriptions = %d total...", cfg.c, cfg.s, cfg.c*cfg.s)
	pool, err := lib.BuildPool(lib.PoolConfig{
		ServerURL:   server.WebSocketURL(),
		C:           cfg.c,
		S:           cfg.s,
		BigS:        cfg.bigS,
		Tolerance:   cfg.tolerance,
		Seed:        cfg.seed,
		Accumulator: acc,
		Streams:     opts.streamSubsets,
		OnConnect:   func() { connected.Add(1) },
		OnSubscribe: func() { subscribed.Add(1) },
	})
	if err != nil {
		fmt.Fprintln(stdout, "pool setup failed:", err)
		return 1
	}
	defer pool.Close()
	u.logf("pool ready: %d connected, %d subscribed (%s)",
		connected.Load(), subscribed.Load(),
		time.Since(poolStart).Round(10*time.Millisecond))

	broadcastURL := server.BroadcastURL()
	if opts.broadcastURL != "" {
		broadcastURL = opts.broadcastURL
	}
	pub := lib.NewPublisher(broadcastURL, cfg.maxInflight,
		lib.WithWorkers(cfg.publishWorkers),
		lib.WithBatchSize(cfg.publishBatch),
	)
	defer pub.Close()

	if opts.setupHook != nil {
		opts.setupHook(server, pool, pub)
	}

	totalSchedule := int64(cfg.d / publishInterval(cfg))
	expectedRate := float64(cfg.r) * float64(cfg.c) * float64(cfg.s) / float64(cfg.bigS)
	expectedReceived := int64(float64(totalSchedule) * float64(cfg.c) * float64(cfg.s) / float64(cfg.bigS))
	u.logf("expected throughput: %s msg/s (r × c × s / S = %d × %d × %d / %d)",
		formatRate(expectedRate), cfg.r, cfg.c, cfg.s, cfg.bigS)
	u.logf("starting publish window: %dHz for %s (%d scheduled publishes)...",
		cfg.r, cfg.d, totalSchedule)

	// Two samplers run in parallel, both ticking at progressTickInterval so
	// the timelines align. recvSampler covers publish + drain (the receive
	// rate keeps mattering until the last message lands); pubSampler runs
	// only while POSTs are being completed (publishStart -> pub.Close
	// returns) so the rate distribution isn't diluted by a flat tail of
	// post-drain zero samples.
	recvSampler := lib.NewReceiveSampler(progressTickInterval, acc.TotalReceived)
	pubSampler := lib.NewReceiveSampler(progressTickInterval, pub.CompletedCount)
	recvSampler.Start()
	pubSampler.Start()

	// Two progress bars updated on the same tick: publishing tracks
	// completed POSTs (real backpressure visible after publishWindow
	// returns); receiving tracks total received messages so the operator
	// sees the fan-out happening in real time. No intermediate log line
	// between publishWindow and Close — those would collide with the
	// still-rendering bars.
	progress := u.startProgress(
		bar{label: "publishing", total: totalSchedule, get: pub.CompletedCount},
		bar{label: "receiving", total: expectedReceived, get: acc.TotalReceived},
	)
	publishStart := time.Now()
	late := publishWindow(cfg, pub)
	pub.Close()
	publishWallSecs := time.Since(publishStart).Seconds()
	pubSampler.Stop()

	targets := pub.TargetCounts()
	expected := computeExpected(pool, targets)

	drainStart := time.Now()
	pollDrain(acc, expected, cfg.drainTimeout)
	drainElapsed := time.Since(drainStart)
	recvSampler.Stop()
	progress.Stop()
	u.logf("publish window done: %d issued, %d completed, %d late, %d dropped in %.2fs (%.0f msg/s actual)",
		pub.IssuedCount(), pub.CompletedCount(), late, pub.DroppedCount(),
		publishWallSecs, float64(pub.CompletedCount())/publishWallSecs)
	u.logf("drain complete (%s)", drainElapsed.Round(10*time.Millisecond))

	snap := acc.Snapshot()
	recvSamples := recvSampler.Samples()
	pubSamples := pubSampler.Samples()
	throughputWS := lib.ComputeWindowStats(recvSamples, cfg.d)
	throughputOverall := lib.Overall(recvSamples)
	publishWS := lib.ComputeWindowStats(pubSamples, cfg.d)
	s := buildSummary(cfg, snap, expected, pub, late, publishWallSecs,
		throughputOverall, throughputWS, publishWS, expectedRate)
	u.logf("summary:")
	printSummary(stdout, s)

	if s.clientsShort > 0 || s.publicationsDropped > 0 {
		return 1
	}
	return 0
}

// publishInterval mirrors publishWindow's interval computation so the UI can
// pre-compute the total scheduled publishes for the progress bar denominator.
func publishInterval(cfg runConfig) time.Duration {
	interval := time.Second / time.Duration(cfg.r)
	if interval <= 0 {
		interval = time.Nanosecond
	}
	return interval
}

// publishWindow walks the scheduled tick grid in absolute time. For tick i,
// the scheduled instant is start + i*interval; the loop sleeps to that
// instant before firing, except when the wallclock is already past it — in
// which case it fires immediately and increments latePublishes. Total
// issued count is deterministic for a given (r, d): every nextTickAt <= end
// fires exactly once, regardless of OS scheduler jitter. Catch-up bursts
// can extend wallclock past d; runWithConfig's drainTimeout bounds the
// outer run.
func publishWindow(cfg runConfig, pub *lib.Publisher) (latePublishes int) {
	rng := rand.New(rand.NewPCG(cfg.seed^0xA5A5A5A5A5A5A5A5, cfg.seed))
	interval := publishInterval(cfg)
	start := time.Now()
	end := start.Add(cfg.d)
	for i := int64(1); ; i++ {
		nextTickAt := start.Add(time.Duration(i) * interval)
		if nextTickAt.After(end) {
			return
		}
		now := time.Now()
		if now.Before(nextTickAt) {
			time.Sleep(nextTickAt.Sub(now))
		} else {
			latePublishes++
		}
		stream := strconv.Itoa(rng.IntN(cfg.bigS) + 1)
		pub.Publish(stream, "x")
	}
}

func computeExpected(pool *lib.ClientPool, targets map[string]int) map[string]int {
	subsets := pool.Streams()
	out := make(map[string]int, len(subsets))
	for i, streams := range subsets {
		id := strconv.Itoa(i)
		// A client subscribed to N copies of the same stream would still
		// receive each broadcast once. Subsets are generated without
		// replacement, so this is moot in production, but we dedupe
		// defensively in case a test injects Streams with duplicates.
		seen := make(map[string]struct{}, len(streams))
		exp := 0
		for _, s := range streams {
			if _, dup := seen[s]; dup {
				continue
			}
			seen[s] = struct{}{}
			exp += targets[s]
		}
		out[id] = exp
	}
	return out
}

func pollDrain(acc *lib.Accumulator, expected map[string]int, timeout time.Duration) {
	deadline := time.After(timeout)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		snap := acc.Snapshot()
		if drained(snap, expected) {
			return
		}
		select {
		case <-tick.C:
		case <-deadline:
			return
		}
	}
}

func drained(snap, expected map[string]int) bool {
	for id, exp := range expected {
		if snap[id] < exp {
			return false
		}
	}
	return true
}

func buildSummary(cfg runConfig, snap, expected map[string]int, pub *lib.Publisher, latePublishes int, publishWallSecs, throughputOverall float64, throughputWS, publishWS lib.WindowStats, expectedRate float64) summary {
	var (
		clientsComp   int
		clientsShort  int
		messagesShort int
	)
	for id, exp := range expected {
		got := snap[id]
		switch {
		case got >= exp:
			clientsComp++
		default:
			clientsShort++
			messagesShort += exp - got
		}
	}
	observedRate := 0.0
	if publishWallSecs > 0 {
		observedRate = float64(pub.CompletedCount()) / publishWallSecs
	}
	return summary{
		throughputMsgsPerSec:    throughputOverall,
		throughputMaxMsgsPerSec: throughputWS.Max,
		throughputMinMsgsPerSec: throughputWS.Min,
		throughputP50MsgsPerSec: throughputWS.P50,
		throughputP95MsgsPerSec: throughputWS.P95,
		expectedMsgsPerSec:      expectedRate,
		clientsComplete:         clientsComp,
		clientsShort:            clientsShort,
		messagesMissing:         messagesShort,
		publicationsIssued:      pub.IssuedCount(),
		publicationsCompleted:   pub.CompletedCount(),
		publicationsDropped:     pub.DroppedCount(),
		schedulerLatePublishes:  latePublishes,
		observedPublishRate:     observedRate,
		observedPublishRateMax:  publishWS.Max,
		observedPublishRateMin:  publishWS.Min,
		observedPublishRateP50:  publishWS.P50,
		observedPublishRateP95:  publishWS.P95,
		publishWindowWallSecs:   publishWallSecs,
	}
}

func printSummary(w io.Writer, s summary) {
	fmt.Fprintf(w, "throughput_msgs_per_sec=%.2f\n", s.throughputMsgsPerSec)
	fmt.Fprintf(w, "throughput_max_msgs_per_sec=%.2f\n", s.throughputMaxMsgsPerSec)
	fmt.Fprintf(w, "throughput_min_msgs_per_sec=%.2f\n", s.throughputMinMsgsPerSec)
	fmt.Fprintf(w, "throughput_p50_msgs_per_sec=%.2f\n", s.throughputP50MsgsPerSec)
	fmt.Fprintf(w, "throughput_p95_msgs_per_sec=%.2f\n", s.throughputP95MsgsPerSec)
	fmt.Fprintf(w, "expected_msgs_per_sec=%.2f\n", s.expectedMsgsPerSec)
	fmt.Fprintf(w, "clients_complete=%d\n", s.clientsComplete)
	fmt.Fprintf(w, "clients_short=%d\n", s.clientsShort)
	fmt.Fprintf(w, "messages_missing=%d\n", s.messagesMissing)
	fmt.Fprintf(w, "publications_issued=%d\n", s.publicationsIssued)
	fmt.Fprintf(w, "publications_completed=%d\n", s.publicationsCompleted)
	fmt.Fprintf(w, "publications_dropped=%d\n", s.publicationsDropped)
	fmt.Fprintf(w, "scheduler_late_publishes=%d\n", s.schedulerLatePublishes)
	fmt.Fprintf(w, "observed_publish_rate=%.2f\n", s.observedPublishRate)
	fmt.Fprintf(w, "observed_publish_rate_max=%.2f\n", s.observedPublishRateMax)
	fmt.Fprintf(w, "observed_publish_rate_min=%.2f\n", s.observedPublishRateMin)
	fmt.Fprintf(w, "observed_publish_rate_p50=%.2f\n", s.observedPublishRateP50)
	fmt.Fprintf(w, "observed_publish_rate_p95=%.2f\n", s.observedPublishRateP95)
	fmt.Fprintf(w, "publish_window_wall_seconds=%.2f\n", s.publishWindowWallSecs)
}
