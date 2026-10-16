// reconnect_floods reproduces reconnect storms against an RPC-backed
// AnyCable server: C clients connect and subscribe through a slow fake HTTP
// RPC, then waves of clients lose their connections at once and reconnect.
// Clients that miss the subscribe or ping deadline reconnect too, so a
// large enough wave can feed itself. The scenario reports whether every
// client returns to the healthy state and how long handshakes and
// subscription confirmations take on the way.
package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/anycable/anycable-go/config"
	"github.com/anycable/anycable-go/etc/benchi/lib"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type runConfig struct {
	c                 int
	s                 int
	waves             []int
	waveInterval      time.Duration
	rampRate          int
	setupTimeout      time.Duration
	settle            time.Duration
	recoveryTimeout   time.Duration
	rpcLatency        time.Duration
	rpcJitter         float64
	connectLatency    time.Duration
	commandLatency    time.Duration
	disconnectLatency time.Duration
	rpcConcurrency    int
	rpcWaitTimeout    time.Duration
	disconnectRate    int
	disconnectBacklog int
	pingInterval      int
	pingTimeout       time.Duration
	sequentialSub     bool
	subRetryInterval  time.Duration
	subRetriesMax     int
	backoffMin        time.Duration
	backoffMax        time.Duration
	seed              uint64
	nonInteractive    bool
	timeline          string
	serverLog         string
}

func run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseFlags(args, stderr)
	if err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(stderr, "error:", err)
		}
		return 2
	}

	// The first Ctrl-C (or SIGTERM) stops the run and prints the summary.
	// The handler is then removed, so a second Ctrl-C kills the process.
	runCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	go func() {
		<-runCtx.Done()
		stopSignals()
	}()

	return runWithConfig(runCtx, cfg, stdout, stderr)
}

func parseFlags(args []string, stderr io.Writer) (runConfig, error) {
	fs := flag.NewFlagSet("reconnect_floods", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var cfg runConfig
	var waves string
	var seed int64
	fs.IntVar(&cfg.c, "c", 0, "number of clients (required, > 0)")
	fs.IntVar(&cfg.s, "s", 3, "RPC-backed subscriptions per client")
	fs.StringVar(&waves, "waves", "", "comma-separated wave sizes: clients dropped at once per wave, e.g. 1000,1000,3000 (required)")
	fs.DurationVar(&cfg.waveInterval, "wave-interval", 60*time.Second, "time between waves")
	fs.IntVar(&cfg.rampRate, "ramp-rate", 200, "initial connects per second during setup")
	fs.DurationVar(&cfg.setupTimeout, "setup-timeout", 2*time.Minute, "max time for all clients to become healthy before the first wave")
	fs.DurationVar(&cfg.settle, "settle", 5*time.Second, "steady-state time between setup and the first wave")
	fs.DurationVar(&cfg.recoveryTimeout, "recovery-timeout", 5*time.Minute, "max time after the last wave for all clients to become healthy (the run then waits for the disconnect queue to drain)")
	fs.DurationVar(&cfg.rpcLatency, "rpc-latency", 100*time.Millisecond, "default latency of every fake RPC call")
	fs.Float64Var(&cfg.rpcJitter, "rpc-jitter", 0.3, "RPC latency jitter as a fraction: each call takes latency × (1 ± jitter)")
	fs.DurationVar(&cfg.connectLatency, "rpc-connect-latency", 0, "Connect RPC latency (0 = --rpc-latency)")
	fs.DurationVar(&cfg.commandLatency, "rpc-command-latency", 0, "Command RPC latency (0 = --rpc-latency)")
	fs.DurationVar(&cfg.disconnectLatency, "rpc-disconnect-latency", 0, "Disconnect RPC latency (0 = --rpc-latency)")
	fs.IntVar(&cfg.rpcConcurrency, "rpc-concurrency", 28, "server: rpc_concurrency (shared by all RPC methods)")
	fs.DurationVar(&cfg.rpcWaitTimeout, "rpc-wait-timeout", -1, "server: rpc_wait_timeout (max wait for a free RPC slot; negative = command timeout, 0 = no limit)")
	fs.IntVar(&cfg.disconnectRate, "disconnect-rate", 100, "server: disconnect_rate (Disconnect calls per second)")
	fs.IntVar(&cfg.disconnectBacklog, "disconnect-backlog", 8192, "server: max_disconnect_backlog_size (Disconnect calls over the limit are discarded)")
	fs.IntVar(&cfg.pingInterval, "ping-interval", 3, "server: ping_interval in seconds")
	fs.DurationVar(&cfg.pingTimeout, "ping-timeout", 6*time.Second, "client: reconnect when no ping arrives for this long (watch starts at socket open)")
	fs.BoolVar(&cfg.sequentialSub, "sequential-subscribe", false, "client: send the next subscribe command only after the previous one is confirmed")
	fs.DurationVar(&cfg.subRetryInterval, "sub-retry-interval", 5*time.Second, "client: re-send a subscribe command when it is not confirmed for this long")
	fs.IntVar(&cfg.subRetriesMax, "sub-retries-max", 2, "client: re-sends per channel; reconnect when the last one is not confirmed in --sub-retry-interval")
	fs.DurationVar(&cfg.backoffMin, "backoff-min", 500*time.Millisecond, "client: first reconnect delay; doubles per failed attempt, ±50% jitter")
	fs.DurationVar(&cfg.backoffMax, "backoff-max", 10*time.Second, "client: max reconnect delay")
	fs.Int64Var(&seed, "seed", 1, "RNG seed for wave selection and backoff jitter")
	fs.StringVar(&cfg.serverLog, "server-log", "", "if set, write the server debug log to this path (large: ~1KB per connection attempt)")
	fs.StringVar(&cfg.timeline, "timeline", "", "if set, write a per-second CSV timeline (client states, RPC calls/s, server gauges, timings) to this path")
	fs.BoolVar(&cfg.nonInteractive, "non-interactive", false, "suppress the live dashboard on stderr; emit only the key=value summary on stdout (auto-on when stderr is not a TTY)")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	cfg.seed = uint64(seed) //nolint:gosec // operator-supplied seed

	if cfg.c <= 0 || waves == "" {
		fs.Usage()
		return cfg, errors.New("required flags missing: -c must be > 0 and --waves must be set")
	}
	for _, part := range strings.Split(waves, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n <= 0 {
			return cfg, fmt.Errorf("--waves: invalid wave size %q", part)
		}
		if n > cfg.c {
			return cfg, fmt.Errorf("--waves: wave size %d exceeds -c %d", n, cfg.c)
		}
		cfg.waves = append(cfg.waves, n)
	}
	if cfg.s < 0 || cfg.rampRate <= 0 || cfg.rpcConcurrency <= 0 || cfg.disconnectRate <= 0 || cfg.disconnectBacklog <= 0 {
		return cfg, errors.New("-s, --ramp-rate, --rpc-concurrency, --disconnect-rate, --disconnect-backlog must be positive")
	}
	if cfg.subRetryInterval <= 0 || cfg.subRetriesMax < 0 {
		return cfg, errors.New("--sub-retry-interval must be positive and --sub-retries-max must not be negative")
	}
	if cfg.rpcJitter < 0 || cfg.rpcJitter > 1 {
		return cfg, errors.New("--rpc-jitter must be in [0, 1]")
	}
	return cfg, nil
}

// waveResult records one wave's outcome. recovery is the time until all
// clients became healthy. drained is the time until the disconnect queue
// became empty after that. Both are -1 when it did not happen before the
// next wave (or the recovery timeout for recovery, the interrupt for drained).
type waveResult struct {
	size     int
	dropped  int
	firedAt  time.Duration
	recovery time.Duration
	drained  time.Duration
}

// runWithConfig runs the scenario until it completes or runCtx is done.
// When runCtx is done, it stops waiting and prints the summary so far.
func runWithConfig(runCtx context.Context, cfg runConfig, stdout, stderr io.Writer) int {
	start := time.Now()
	u := newUI(stderr, cfg.nonInteractive, start)

	rpc := lib.NewRPCServer(lib.RPCServerConfig{
		Latency:           cfg.rpcLatency,
		ConnectLatency:    cfg.connectLatency,
		CommandLatency:    cfg.commandLatency,
		DisconnectLatency: cfg.disconnectLatency,
		Jitter:            cfg.rpcJitter,
	})
	defer rpc.Close()

	var serverLogger *slog.Logger
	if cfg.serverLog != "" {
		f, err := os.Create(cfg.serverLog)
		if err != nil {
			fmt.Fprintln(stdout, "server log:", err)
			return 1
		}
		defer f.Close()
		serverLogger = slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	server, err := lib.BuildServer(lib.ServerConfig{
		Logger:  serverLogger,
		RPCHost: rpc.URL(),
		Configure: func(c *config.Config) {
			c.RPC.Concurrency = cfg.rpcConcurrency
			c.RPC.WaitTimeout = int(cfg.rpcWaitTimeout.Milliseconds())
			if cfg.rpcWaitTimeout < 0 {
				c.RPC.WaitTimeout = -1
			}
			c.DisconnectQueue.Rate = cfg.disconnectRate
			c.DisconnectQueue.MaxBacklog = cfg.disconnectBacklog
			c.App.PingInterval = cfg.pingInterval
			c.App.StatsRefreshInterval = 1
		},
	})
	if err != nil {
		fmt.Fprintln(stdout, "server build failed:", err)
		return 1
	}

	stats := &lib.ClientStats{}
	channels := make([]string, cfg.s)
	for i := range channels {
		channels[i] = fmt.Sprintf(`{"channel":"BenchChannel","id":"%d"}`, i+1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	clients := make([]*lib.ReconnectingClient, cfg.c)
	for i := range clients {
		clients[i] = lib.NewReconnectingClient(lib.ReconnectingClientConfig{
			URL:                    server.WebSocketURL(),
			Channels:               channels,
			SequentialSubscribe:    cfg.sequentialSub,
			SubscribeRetryInterval: cfg.subRetryInterval,
			SubscribeRetriesMax:    cfg.subRetriesMax,
			PingTimeout:            cfg.pingTimeout,
			Backoff:                lib.Backoff{Min: cfg.backoffMin, Max: cfg.backoffMax},
			Stats:                  stats,
			Seed:                   cfg.seed + uint64(i), //nolint:gosec // i >= 0
		})
		delay := time.Duration(i) * time.Second / time.Duration(cfg.rampRate)
		wg.Add(1)
		go func(c *lib.ReconnectingClient) {
			defer wg.Done()
			c.Run(ctx, delay)
		}(clients[i])
	}

	smp := newSampler(start, rpc, server, stats)
	smp.start()

	phase := newPhase("setup", fmt.Sprintf("connecting %d clients at %d/s", cfg.c, cfg.rampRate))
	dash := u.startDashboard(func() []string { return renderDashboard(cfg, smp, phase.get()) })

	waves := make([]waveResult, 0, len(cfg.waves))
	recovered := false
	setupOK := waitHealthy(runCtx, stats, cfg.c, cfg.setupTimeout)
	if !setupOK && runCtx.Err() != nil {
		u.logf("interrupted during setup: %d/%d clients healthy", stats.State(lib.StateHealthy), cfg.c)
	} else if !setupOK {
		u.logf("setup failed: %d/%d clients healthy after %s", stats.State(lib.StateHealthy), cfg.c, cfg.setupTimeout)
	} else {
		u.logf("setup done: %d clients healthy in %s", cfg.c, time.Since(start).Round(100*time.Millisecond))
		phase.set("settle", fmt.Sprintf("steady state for %s", cfg.settle))
		select {
		case <-time.After(cfg.settle):
		case <-runCtx.Done():
		}

		rng := rand.New(rand.NewPCG(cfg.seed, cfg.seed^0xA5A5A5A5A5A5A5A5))
		for i, size := range cfg.waves {
			if runCtx.Err() != nil {
				break
			}
			last := i == len(cfg.waves)-1
			droppedBefore := stats.Reason(lib.ReasonDropped)
			fired := time.Now()
			dropped := dropWave(clients, size, rng)
			w := waveResult{size: size, dropped: dropped, firedAt: fired.Sub(start), recovery: -1, drained: -1}
			u.logf("wave %d/%d: dropped %d clients (requested %d)", i+1, len(cfg.waves), dropped, size)
			phase.set(fmt.Sprintf("wave %d/%d", i+1, len(cfg.waves)),
				fmt.Sprintf("fired at %.0fs, %d dropped", w.firedAt.Seconds(), dropped))

			// After the last wave recovers, the run keeps going until the
			// disconnect queue is empty (or Ctrl-C): the disconnector makes one
			// call at a time, so draining can take long.
			wait := cfg.waveInterval
			if last {
				wait = cfg.recoveryTimeout
			}
			deadline := fired.Add(wait)
			for runCtx.Err() == nil && (time.Now().Before(deadline) || (last && w.recovery >= 0)) {
				if w.recovery < 0 &&
					stats.Reason(lib.ReasonDropped)-droppedBefore >= int64(dropped) &&
					stats.State(lib.StateHealthy) == int64(cfg.c) {
					w.recovery = time.Since(fired)
					u.logf("wave %d/%d: all clients healthy after %s", i+1, len(cfg.waves), w.recovery.Round(100*time.Millisecond))
					status := fmt.Sprintf("recovered in %.1fs", w.recovery.Seconds())
					if last {
						status += ", waiting for the disconnect queue to drain (Ctrl-C to stop)"
					}
					phase.set(fmt.Sprintf("wave %d/%d", i+1, len(cfg.waves)), status)
				}
				if w.recovery >= 0 && w.drained < 0 && server.DisconnectQueueSize() == 0 {
					w.drained = time.Since(fired)
					u.logf("wave %d/%d: disconnect queue empty after %s", i+1, len(cfg.waves), w.drained.Round(100*time.Millisecond))
					if last {
						break
					}
				}
				time.Sleep(100 * time.Millisecond)
			}
			if runCtx.Err() != nil {
				u.logf("wave %d/%d: interrupted after %s (clients healthy: %t, disconnect queue: %d)",
					i+1, len(cfg.waves), time.Since(fired).Round(100*time.Millisecond), w.recovery >= 0, server.DisconnectQueueSize())
			} else if w.recovery < 0 {
				u.logf("wave %d/%d: not recovered after %s", i+1, len(cfg.waves), wait)
			}
			waves = append(waves, w)
		}
		// Waves left out by an interrupt do not count as recovered.
		recovered = len(waves) == len(cfg.waves) && waves[len(waves)-1].recovery >= 0
	}
	interrupted := runCtx.Err() != nil

	smp.stop()
	dash.Stop()
	sum := buildSummary(cfg, setupOK, recovered, time.Since(start), waves, stats, rpc, smp)
	sum.interrupted = interrupted
	printSummary(stdout, sum)
	if cfg.timeline != "" {
		if err := writeTimeline(cfg.timeline, smp.all()); err != nil {
			fmt.Fprintln(stdout, "timeline write failed:", err)
		}
	}

	cancel()
	wg.Wait()
	// After a storm, thousands of closed sessions can wait in the disconnect
	// queue; a graceful shutdown would make each Disconnect call in turn.
	// The run is over, so bound the teardown instead.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)

	if interrupted {
		return 130
	}
	if !setupOK || !recovered {
		return 1
	}
	return 0
}

// dropWave abruptly closes up to size open connections, chosen at random.
// Clients that are between connections (backoff) are skipped, so the
// result can be lower than size during a storm.
func dropWave(clients []*lib.ReconnectingClient, size int, rng *rand.Rand) int {
	dropped := 0
	for _, i := range rng.Perm(len(clients)) {
		if dropped == size {
			break
		}
		if clients[i].Drop() {
			dropped++
		}
	}
	return dropped
}

func waitHealthy(ctx context.Context, stats *lib.ClientStats, c int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) && ctx.Err() == nil {
		if stats.State(lib.StateHealthy) == int64(c) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// phase is the dashboard's "what is happening now" line.
type phase struct {
	mu          sync.Mutex
	name, label string
}

func newPhase(name, label string) *phase { return &phase{name: name, label: label} }

func (p *phase) set(name, label string) {
	p.mu.Lock()
	p.name, p.label = name, label
	p.mu.Unlock()
}

func (p *phase) get() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.name + ": " + p.label
}

// sample is one 1-second observation. Counters are cumulative; rates are
// computed against the previous sample.
type sample struct {
	at          time.Duration
	states      [4]int64
	rpcStarted  [3]int64
	rpcDone     [3]int64
	rpcInFlight int64
	rpcPending  uint64
	rpcCanceled uint64
	rpcWaitTO   uint64
	sessions    uint64
	discQueue   uint64
	goroutines  int
	heapBytes   uint64
	reasons     map[lib.CloseReason]int64
	attempts    int64
	reconnects  int64
	subRetries  int64
	welcome     lib.HistogramSnapshot
	confirm     lib.HistogramSnapshot
}

type sampler struct {
	begin  time.Time
	rpc    *lib.RPCServer
	server *lib.Server
	stats  *lib.ClientStats

	mu      sync.Mutex
	samples []sample
	done    chan struct{}
	exit    chan struct{}
}

func newSampler(begin time.Time, rpc *lib.RPCServer, server *lib.Server, stats *lib.ClientStats) *sampler {
	return &sampler{begin: begin, rpc: rpc, server: server, stats: stats, done: make(chan struct{}), exit: make(chan struct{})}
}

func (s *sampler) start() {
	s.record()
	go func() {
		defer close(s.exit)
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				s.record()
			case <-s.done:
				return
			}
		}
	}()
}

func (s *sampler) stop() {
	close(s.done)
	<-s.exit
	s.record()
}

func (s *sampler) record() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	smp := sample{
		at:         time.Since(s.begin),
		rpcPending: s.server.Gauge("rpc_pending_num"),
		// Calls that waited for an RPC slot and were dropped because the
		// session had closed meanwhile; they never reach the RPC server.
		rpcCanceled: s.server.Counter("rpc_canceled_total"),
		// Calls that waited for an RPC slot longer than rpc_wait_timeout;
		// they never reach the RPC server either.
		rpcWaitTO:  s.server.Counter("rpc_wait_timeout_total"),
		sessions:   s.server.Gauge("clients_num"),
		discQueue:  uint64(s.server.DisconnectQueueSize()), //nolint:gosec // size >= 0
		goroutines: runtime.NumGoroutine(),
		heapBytes:  ms.HeapInuse,
		reasons:    make(map[lib.CloseReason]int64),
		attempts:   s.stats.Attempts(),
		subRetries: s.stats.SubscribeRetries(),
		reconnects: s.stats.Reconnects(),
		welcome:    s.stats.Welcome.Snapshot(),
		confirm:    s.stats.Confirm.Snapshot(),
	}
	for st := lib.StateConnecting; st <= lib.StateBackoff; st++ {
		smp.states[st] = s.stats.State(st)
	}
	for m := lib.RPCConnect; m <= lib.RPCDisconnect; m++ {
		ms := s.rpc.Stats(m)
		smp.rpcStarted[m] = ms.Started
		smp.rpcDone[m] = ms.Completed
		smp.rpcInFlight += ms.InFlight
	}
	for _, r := range lib.AllCloseReasons() {
		smp.reasons[r] = s.stats.Reason(r)
	}
	s.mu.Lock()
	s.samples = append(s.samples, smp)
	s.mu.Unlock()
}

// window returns the latest sample and the one n seconds earlier (or the
// oldest available).
func (s *sampler) window(n int) (cur, prev sample, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.samples) < 2 {
		return sample{}, sample{}, false
	}
	cur = s.samples[len(s.samples)-1]
	i := len(s.samples) - 1 - n
	if i < 0 {
		i = 0
	}
	return cur, s.samples[i], true
}

func (s *sampler) all() []sample {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]sample(nil), s.samples...)
}

// commandStall returns the current and the longest run of consecutive
// samples in which clients waited for confirmations but no Command RPC
// call completed. This is the "subscribe never reached RPC" signal.
func commandStall(samples []sample) (current, longest time.Duration) {
	var runStart time.Duration = -1
	for i := 1; i < len(samples); i++ {
		cur, prev := samples[i], samples[i-1]
		stalled := cur.states[lib.StateSubscribing] > 0 && cur.rpcDone[lib.RPCCommand] == prev.rpcDone[lib.RPCCommand]
		switch {
		case stalled && runStart < 0:
			runStart = prev.at
		case !stalled:
			runStart = -1
		}
		if runStart >= 0 {
			current = cur.at - runStart
			if current > longest {
				longest = current
			}
		} else {
			current = 0
		}
	}
	return current, longest
}

type summary struct {
	setupOK, recovered bool
	interrupted        bool
	clients            int
	runSeconds         float64
	waves              []waveResult
	attempts           int64
	reconnects         int64
	subRetries         int64
	reasons            map[lib.CloseReason]int64
	rpcCalls           [3]int64
	maxCommandStall    time.Duration
	welcome, confirm   lib.HistogramSnapshot
	healthy            lib.HistogramSnapshot
	peakDiscQueue      uint64
	peakRPCPending     uint64
	rpcCanceled        uint64
	rpcWaitTO          uint64
	peakSessions       uint64
	peakGoroutines     int
	peakHeapBytes      uint64
}

func buildSummary(cfg runConfig, setupOK, recovered bool, elapsed time.Duration, waves []waveResult, stats *lib.ClientStats, rpc *lib.RPCServer, smp *sampler) summary {
	s := summary{
		setupOK:    setupOK,
		recovered:  recovered,
		clients:    cfg.c,
		runSeconds: elapsed.Seconds(),
		waves:      waves,
		attempts:   stats.Attempts(),
		reconnects: stats.Reconnects(),
		subRetries: stats.SubscribeRetries(),
		reasons:    make(map[lib.CloseReason]int64),
		welcome:    stats.Welcome.Snapshot(),
		confirm:    stats.Confirm.Snapshot(),
		healthy:    stats.Healthy.Snapshot(),
	}
	for _, r := range lib.AllCloseReasons() {
		s.reasons[r] = stats.Reason(r)
	}
	for m := lib.RPCConnect; m <= lib.RPCDisconnect; m++ {
		s.rpcCalls[m] = rpc.Stats(m).Completed
	}
	samples := smp.all()
	if len(samples) > 0 {
		s.rpcCanceled = samples[len(samples)-1].rpcCanceled
		s.rpcWaitTO = samples[len(samples)-1].rpcWaitTO
	}
	_, s.maxCommandStall = commandStall(samples)
	for _, x := range samples {
		s.peakDiscQueue = max(s.peakDiscQueue, x.discQueue)
		s.peakRPCPending = max(s.peakRPCPending, x.rpcPending)
		s.peakSessions = max(s.peakSessions, x.sessions)
		s.peakGoroutines = max(s.peakGoroutines, x.goroutines)
		s.peakHeapBytes = max(s.peakHeapBytes, x.heapBytes)
	}
	return s
}

func printSummary(w io.Writer, s summary) {
	fmt.Fprintf(w, "setup_ok=%t\n", s.setupOK)
	fmt.Fprintf(w, "recovered=%t\n", s.recovered)
	fmt.Fprintf(w, "interrupted=%t\n", s.interrupted)
	fmt.Fprintf(w, "clients=%d\n", s.clients)
	fmt.Fprintf(w, "run_seconds=%.2f\n", s.runSeconds)
	fmt.Fprintf(w, "waves_fired=%d\n", len(s.waves))
	for i, wv := range s.waves {
		fmt.Fprintf(w, "wave_%d_size=%d\n", i+1, wv.size)
		fmt.Fprintf(w, "wave_%d_dropped=%d\n", i+1, wv.dropped)
		fmt.Fprintf(w, "wave_%d_fired_at_seconds=%.2f\n", i+1, wv.firedAt.Seconds())
		fmt.Fprintf(w, "wave_%d_recovery_seconds=%.2f\n", i+1, durOrNeg(wv.recovery))
		fmt.Fprintf(w, "wave_%d_drained_seconds=%.2f\n", i+1, durOrNeg(wv.drained))
	}
	fmt.Fprintf(w, "connection_attempts=%d\n", s.attempts)
	fmt.Fprintf(w, "reconnects=%d\n", s.reconnects)
	fmt.Fprintf(w, "subscribe_retries=%d\n", s.subRetries)
	for _, r := range lib.AllCloseReasons() {
		fmt.Fprintf(w, "close_%s=%d\n", r, s.reasons[r])
	}
	fmt.Fprintf(w, "rpc_connect_calls=%d\n", s.rpcCalls[lib.RPCConnect])
	fmt.Fprintf(w, "rpc_command_calls=%d\n", s.rpcCalls[lib.RPCCommand])
	fmt.Fprintf(w, "rpc_disconnect_calls=%d\n", s.rpcCalls[lib.RPCDisconnect])
	fmt.Fprintf(w, "rpc_canceled_calls=%d\n", s.rpcCanceled)
	fmt.Fprintf(w, "rpc_wait_timeout_calls=%d\n", s.rpcWaitTO)
	fmt.Fprintf(w, "max_command_stall_seconds=%.0f\n", s.maxCommandStall.Seconds())
	printHistogram(w, "welcome", s.welcome)
	printHistogram(w, "confirm", s.confirm)
	printHistogram(w, "healthy", s.healthy)
	fmt.Fprintf(w, "peak_disconnect_queue=%d\n", s.peakDiscQueue)
	fmt.Fprintf(w, "peak_rpc_pending=%d\n", s.peakRPCPending)
	fmt.Fprintf(w, "peak_server_sessions=%d\n", s.peakSessions)
	fmt.Fprintf(w, "peak_goroutines=%d\n", s.peakGoroutines)
	fmt.Fprintf(w, "peak_heap_mb=%.1f\n", float64(s.peakHeapBytes)/(1<<20))
}

func printHistogram(w io.Writer, name string, h lib.HistogramSnapshot) {
	fmt.Fprintf(w, "%s_count=%d\n", name, h.Count)
	fmt.Fprintf(w, "%s_p50_ms=%.1f\n", name, ms(h.Quantile(0.5)))
	fmt.Fprintf(w, "%s_p95_ms=%.1f\n", name, ms(h.Quantile(0.95)))
	fmt.Fprintf(w, "%s_p99_ms=%.1f\n", name, ms(h.Quantile(0.99)))
	fmt.Fprintf(w, "%s_max_ms=%.1f\n", name, ms(h.Max))
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func durOrNeg(d time.Duration) float64 {
	if d < 0 {
		return -1
	}
	return d.Seconds()
}

// writeTimeline writes one CSV row per 1s sample. Rates are per second
// against the previous sample; timings are p50/p99 of the samples
// recorded within that second.
func writeTimeline(path string, samples []sample) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	header := []string{"t", "healthy", "subscribing", "connecting", "backoff",
		"rpc_connect_ps", "rpc_command_ps", "rpc_disconnect_ps", "rpc_canceled_ps", "rpc_wait_timeout_ps", "rpc_in_flight", "rpc_pending",
		"sessions", "disconnect_queue", "goroutines", "heap_mb", "attempts_ps", "sub_retries_ps"}
	for _, r := range lib.AllCloseReasons() {
		header = append(header, "close_"+r.String()+"_ps")
	}
	header = append(header, "welcome_p50_ms", "welcome_p99_ms", "confirm_p50_ms", "confirm_p99_ms")
	_ = w.Write(header)

	itoa := func(v int64) string { return strconv.FormatInt(v, 10) }
	for i := 1; i < len(samples); i++ {
		cur, prev := samples[i], samples[i-1]
		secs := (cur.at - prev.at).Seconds()
		rate := func(a, b int64) string { return strconv.FormatFloat(float64(a-b)/secs, 'f', 1, 64) }
		row := []string{
			strconv.FormatFloat(cur.at.Seconds(), 'f', 0, 64),
			itoa(cur.states[lib.StateHealthy]), itoa(cur.states[lib.StateSubscribing]),
			itoa(cur.states[lib.StateConnecting]), itoa(cur.states[lib.StateBackoff]),
			rate(cur.rpcDone[lib.RPCConnect], prev.rpcDone[lib.RPCConnect]),
			rate(cur.rpcDone[lib.RPCCommand], prev.rpcDone[lib.RPCCommand]),
			rate(cur.rpcDone[lib.RPCDisconnect], prev.rpcDone[lib.RPCDisconnect]),
			rate(int64(cur.rpcCanceled), int64(prev.rpcCanceled)), //nolint:gosec // counters fit in int64
			rate(int64(cur.rpcWaitTO), int64(prev.rpcWaitTO)),     //nolint:gosec // counters fit in int64
			itoa(cur.rpcInFlight), strconv.FormatUint(cur.rpcPending, 10),
			strconv.FormatUint(cur.sessions, 10), strconv.FormatUint(cur.discQueue, 10),
			strconv.Itoa(cur.goroutines), strconv.FormatFloat(float64(cur.heapBytes)/(1<<20), 'f', 1, 64),
			rate(cur.attempts, prev.attempts),
			rate(cur.subRetries, prev.subRetries),
		}
		for _, r := range lib.AllCloseReasons() {
			row = append(row, rate(cur.reasons[r], prev.reasons[r]))
		}
		wl, cf := cur.welcome.Sub(prev.welcome), cur.confirm.Sub(prev.confirm)
		row = append(row,
			strconv.FormatFloat(ms(wl.Quantile(0.5)), 'f', 0, 64), strconv.FormatFloat(ms(wl.Quantile(0.99)), 'f', 0, 64),
			strconv.FormatFloat(ms(cf.Quantile(0.5)), 'f', 0, 64), strconv.FormatFloat(ms(cf.Quantile(0.99)), 'f', 0, 64))
		_ = w.Write(row)
	}
	w.Flush()
	return w.Error()
}
