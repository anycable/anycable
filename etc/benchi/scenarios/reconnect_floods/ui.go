package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-isatty"

	"github.com/anycable/anycable-go/etc/benchi/lib"
)

// ui renders lifecycle logs and a live dashboard to stderr when attached to
// a terminal. In non-interactive mode every method is a no-op, so the
// key=value summary on stdout stays the only output.
type ui struct {
	out         io.Writer
	interactive bool
	start       time.Time

	mu        sync.Mutex
	lastLines int
}

func newUI(stderr io.Writer, forceQuiet bool, start time.Time) *ui {
	return &ui{out: stderr, interactive: !forceQuiet && isTerminal(stderr), start: start}
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd())
}

// logf prints one lifecycle line above the dashboard. The dashboard is
// redrawn below it on the next tick.
func (u *ui) logf(format string, args ...any) {
	if !u.interactive {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.clearFrame()
	fmt.Fprintf(u.out, "[%7.2fs] "+format+"\033[K\n", append([]any{time.Since(u.start).Seconds()}, args...)...)
}

// clearFrame moves the cursor to the top of the last frame and erases it.
// Callers hold u.mu.
func (u *ui) clearFrame() {
	if u.lastLines > 0 {
		fmt.Fprintf(u.out, "\033[%dF\033[J", u.lastLines)
		u.lastLines = 0
	}
}

func (u *ui) drawFrame(lines []string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.clearFrame()
	for _, l := range lines {
		fmt.Fprintf(u.out, "%s\033[K\n", l)
	}
	u.lastLines = len(lines)
}

type dashboard struct {
	done chan struct{}
	exit chan struct{}
}

// startDashboard redraws render() every 500ms until Stop. The last frame
// stays on screen.
func (u *ui) startDashboard(render func() []string) *dashboard {
	d := &dashboard{done: make(chan struct{}), exit: make(chan struct{})}
	if !u.interactive {
		close(d.exit)
		return d
	}
	go func() {
		defer close(d.exit)
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for {
			u.drawFrame(render())
			select {
			case <-t.C:
			case <-d.done:
				u.drawFrame(render())
				return
			}
		}
	}()
	return d
}

func (d *dashboard) Stop() {
	select {
	case <-d.done:
	default:
		close(d.done)
	}
	<-d.exit
}

// dashboardWindow is the window (in 1s samples) for rates and percentiles.
const dashboardWindow = 5

func renderDashboard(cfg runConfig, smp *sampler, phase string) []string {
	cur, prev, ok := smp.window(dashboardWindow)
	if !ok {
		return []string{"starting..."}
	}
	secs := (cur.at - prev.at).Seconds()
	rate := func(a, b int64) string {
		if secs <= 0 {
			return "0"
		}
		return formatCount(float64(a-b) / secs)
	}
	stallNow, stallMax := commandStall(smp.all())

	lines := []string{
		fmt.Sprintf("\033[1m%6.0fs  %s\033[0m", cur.at.Seconds(), phase),
		fmt.Sprintf("clients     healthy %s/%d  subscribing %s  connecting %s  backoff %s  │ connection attempts %s",
			paint(cur.states[lib.StateHealthy] == int64(cfg.c), formatCount(float64(cur.states[lib.StateHealthy]))), cfg.c,
			formatCount(float64(cur.states[lib.StateSubscribing])),
			formatCount(float64(cur.states[lib.StateConnecting])),
			formatCount(float64(cur.states[lib.StateBackoff])),
			formatCount(float64(cur.attempts))),
		fmt.Sprintf("rpc/s       connect %s  command %s  disconnect %s  canceled %s  wait-timeout %s  │ in-flight %d/%d  pending %d",
			rate(cur.rpcDone[lib.RPCConnect], prev.rpcDone[lib.RPCConnect]),
			rate(cur.rpcDone[lib.RPCCommand], prev.rpcDone[lib.RPCCommand]),
			rate(cur.rpcDone[lib.RPCDisconnect], prev.rpcDone[lib.RPCDisconnect]),
			rate(int64(cur.rpcCanceled), int64(prev.rpcCanceled)), //nolint:gosec // counters fit in int64
			rate(int64(cur.rpcWaitTO), int64(prev.rpcWaitTO)),     //nolint:gosec // counters fit in int64
			cur.rpcInFlight, cfg.rpcConcurrency, cur.rpcPending),
		fmt.Sprintf("rpc total   connect %s  command %s  disconnect %s  canceled %s  wait-timeout %s",
			formatCount(float64(cur.rpcDone[lib.RPCConnect])),
			formatCount(float64(cur.rpcDone[lib.RPCCommand])),
			formatCount(float64(cur.rpcDone[lib.RPCDisconnect])),
			formatCount(float64(cur.rpcCanceled)),
			formatCount(float64(cur.rpcWaitTO))),
		fmt.Sprintf("server      sessions %s  disconnect queue %d/%d  goroutines %s  heap %.0fMB",
			formatCount(float64(cur.sessions)), cur.discQueue, cfg.disconnectBacklog,
			formatCount(float64(cur.goroutines)), float64(cur.heapBytes)/(1<<20)),
		renderTimings("welcome", cur.welcome.Sub(prev.welcome)),
		renderTimings("confirm", cur.confirm.Sub(prev.confirm)),
		fmt.Sprintf("reconnect/s %s (total %s)  │ %s", rate(cur.reconnects, prev.reconnects),
			formatCount(float64(cur.reconnects)), renderReasons(cur, prev, secs)),
		fmt.Sprintf("sub retry/s %s (total %s)", rate(cur.subRetries, prev.subRetries), formatCount(float64(cur.subRetries))),
		fmt.Sprintf("cmd stall   %s  (longest %s)", paint(stallNow == 0, fmt.Sprintf("%.0fs", stallNow.Seconds())), fmt.Sprintf("%.0fs", stallMax.Seconds())),
		fmt.Sprintf("\033[2m(rates and timings over the last %ds; totals since start)\033[0m", dashboardWindow),
	}
	return lines
}

func renderTimings(name string, h lib.HistogramSnapshot) string {
	if h.Count == 0 {
		return fmt.Sprintf("%-11s —", name)
	}
	return fmt.Sprintf("%-11s p50 %s  p95 %s  p99 %s  max %s  (n=%d)", name,
		fmtDur(h.Quantile(0.5)), fmtDur(h.Quantile(0.95)), fmtDur(h.Quantile(0.99)), fmtDur(h.Max), h.Count)
}

func renderReasons(cur, prev sample, secs float64) string {
	var parts []string
	for _, r := range lib.AllCloseReasons() {
		total := cur.reasons[r]
		if total == 0 {
			continue
		}
		perSec := 0.0
		if secs > 0 {
			perSec = float64(total-prev.reasons[r]) / secs
		}
		parts = append(parts, fmt.Sprintf("%s %s (total %s)", r, formatCount(perSec), formatCount(float64(total))))
	}
	if len(parts) == 0 {
		return "no closes"
	}
	return strings.Join(parts, "  ")
}

func paint(good bool, s string) string {
	if good {
		return "\033[32m" + s + "\033[0m"
	}
	return "\033[31m" + s + "\033[0m"
}

func fmtDur(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", ms(d))
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func formatCount(v float64) string {
	switch {
	case v >= 10_000:
		return fmt.Sprintf("%.1fk", v/1_000)
	case v >= 100 || v == float64(int64(v)):
		return fmt.Sprintf("%.0f", v)
	default:
		return fmt.Sprintf("%.1f", v)
	}
}
