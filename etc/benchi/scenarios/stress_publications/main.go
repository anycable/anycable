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
	c            int
	r            int
	d            time.Duration
	bigS         int
	s            int
	drainTimeout time.Duration
	maxInflight  int
	tolerance    int
	seed         uint64
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

	return runWithConfig(cfg, opts, stdout)
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
	fs.IntVar(&cfg.tolerance, "setup-failure-tolerance", 0, "max client setup failures before aborting")
	var seed int64
	fs.Int64Var(&seed, "seed", 1, "RNG seed for stream-subset selection and per-tick stream choice")

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
type summary struct {
	throughputMsgsPerSec float64
	clientsComplete      int
	clientsShort         int
	messagesMissing      int
	publicationsIssued   int64
	publicationsDropped  int64
	observedPublishRate  float64
}

func runWithConfig(cfg runConfig, opts runOpts, stdout io.Writer) int {
	ctx := context.Background()

	server, err := lib.BuildServer(lib.ServerConfig{})
	if err != nil {
		fmt.Fprintln(stdout, "server build failed:", err)
		return 1
	}
	defer server.Shutdown(ctx) //nolint:errcheck // best-effort shutdown

	acc := lib.NewAccumulator()
	pool, err := lib.BuildPool(lib.PoolConfig{
		ServerURL:   server.WebSocketURL(),
		C:           cfg.c,
		S:           cfg.s,
		BigS:        cfg.bigS,
		Tolerance:   cfg.tolerance,
		Seed:        cfg.seed,
		Accumulator: acc,
		Streams:     opts.streamSubsets,
	})
	if err != nil {
		fmt.Fprintln(stdout, "pool setup failed:", err)
		return 1
	}
	defer pool.Close()

	broadcastURL := server.BroadcastURL()
	if opts.broadcastURL != "" {
		broadcastURL = opts.broadcastURL
	}
	pub := lib.NewPublisher(broadcastURL, cfg.maxInflight)
	defer pub.Close()

	if opts.setupHook != nil {
		opts.setupHook(server, pool, pub)
	}

	publishWindow(cfg, pub)

	// Close the publisher to drain its in-flight queue before reading the
	// target counts. TargetCounts is a snapshot of enqueue-time intent and
	// is stable across Close, but we want all POSTs to have landed at the
	// server before we poll for client-side completeness.
	pub.Close()

	targets := pub.TargetCounts()
	expected := computeExpected(pool, targets)

	pollDrain(acc, expected, cfg.drainTimeout)

	snap := acc.Snapshot()
	s := buildSummary(cfg, snap, expected, pub)
	printSummary(stdout, s)

	if s.clientsShort > 0 || s.publicationsDropped > 0 {
		return 1
	}
	return 0
}

func publishWindow(cfg runConfig, pub *lib.Publisher) {
	rng := rand.New(rand.NewPCG(cfg.seed^0xA5A5A5A5A5A5A5A5, cfg.seed))
	interval := time.Second / time.Duration(cfg.r)
	if interval <= 0 {
		interval = time.Nanosecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	deadline := time.After(cfg.d)
	for {
		select {
		case <-ticker.C:
			stream := strconv.Itoa(rng.IntN(cfg.bigS) + 1)
			pub.Publish(stream, "x")
		case <-deadline:
			return
		}
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

func buildSummary(cfg runConfig, snap, expected map[string]int, pub *lib.Publisher) summary {
	var (
		totalReceived  int
		clientsComp    int
		clientsShort   int
		messagesShort  int
	)
	for id, exp := range expected {
		got := snap[id]
		totalReceived += got
		switch {
		case got >= exp:
			clientsComp++
		default:
			clientsShort++
			messagesShort += exp - got
		}
	}
	seconds := cfg.d.Seconds()
	return summary{
		throughputMsgsPerSec: float64(totalReceived) / seconds,
		clientsComplete:      clientsComp,
		clientsShort:         clientsShort,
		messagesMissing:      messagesShort,
		publicationsIssued:   pub.IssuedCount(),
		publicationsDropped:  pub.DroppedCount(),
		observedPublishRate:  float64(pub.IssuedCount()) / seconds,
	}
}

func printSummary(w io.Writer, s summary) {
	fmt.Fprintf(w, "throughput_msgs_per_sec=%.2f\n", s.throughputMsgsPerSec)
	fmt.Fprintf(w, "clients_complete=%d\n", s.clientsComplete)
	fmt.Fprintf(w, "clients_short=%d\n", s.clientsShort)
	fmt.Fprintf(w, "messages_missing=%d\n", s.messagesMissing)
	fmt.Fprintf(w, "publications_issued=%d\n", s.publicationsIssued)
	fmt.Fprintf(w, "publications_dropped=%d\n", s.publicationsDropped)
	fmt.Fprintf(w, "observed_publish_rate=%.2f\n", s.observedPublishRate)
}
