package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
)

// ui renders lifecycle logs and the publish progress bar to stderr when the
// process is attached to a terminal. When stderr is piped (CI, agents,
// `2>file`) or --non-interactive is set, every method on ui is a no-op so
// the key=value summary on stdout remains the only output — preserving the
// pre-UI behavior contract.
type ui struct {
	out         io.Writer
	interactive bool
	start       time.Time
}

func newUI(stderr io.Writer, forceQuiet bool) *ui {
	return &ui{
		out:         stderr,
		interactive: !forceQuiet && isTerminal(stderr),
		start:       time.Now(),
	}
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd())
}

// logf prints one lifecycle line prefixed with elapsed seconds since
// newUI. Silent in non-interactive mode.
func (u *ui) logf(format string, args ...any) {
	if !u.interactive {
		return
	}
	elapsed := time.Since(u.start).Seconds()
	fmt.Fprintf(u.out, "[%6.2fs] "+format+"\n", append([]any{elapsed}, args...)...)
}

// progress is the handle returned by startProgress. Stop must be called to
// terminate the background renderer and emit the final newline.
type progress struct {
	done chan struct{}
	exit chan struct{}
}

// bar is one row in a multi-bar progress display. getCount is sampled
// each tick and represents the *real* (completed) progress.
type bar struct {
	label string
	total int64
	get   func() int64
}

const (
	progressTickInterval = 100 * time.Millisecond
	// rateWindowSamples = window in 100ms ticks used to compute the
	// instantaneous rate next to each progress bar. 10 samples ≈ 1 second.
	rateWindowSamples = 10
)

// startProgress kicks off a 100ms ticker that rewrites N progress bars on
// stderr, one per row. All bars are updated on the same tick so the
// publishing and receiving views stay in sync. Returns a no-op handle in
// non-interactive mode.
func (u *ui) startProgress(bars ...bar) *progress {
	p := &progress{done: make(chan struct{}), exit: make(chan struct{})}
	if !u.interactive || len(bars) == 0 {
		close(p.exit)
		return p
	}
	go func() {
		defer close(p.exit)
		t := time.NewTicker(progressTickInterval)
		defer t.Stop()
		start := time.Now()
		trackers := make([]*rateTracker, len(bars))
		for i := range bars {
			trackers[i] = newRateTracker(rateWindowSamples)
		}
		first := true
		render := func() {
			now := time.Now()
			lines := make([]string, len(bars))
			for i, b := range bars {
				n := b.get()
				rate := trackers[i].observe(now, n)
				lines[i] = renderBarLine(b.label, n, b.total, time.Since(start), rate)
			}
			renderFrame(u.out, lines, first)
			first = false
		}
		for {
			select {
			case <-t.C:
				render()
			case <-p.done:
				render()
				return
			}
		}
	}()
	return p
}

func (p *progress) Stop() {
	select {
	case <-p.done:
		// already stopped
	default:
		close(p.done)
	}
	<-p.exit
}

// renderFrame writes one frame of the multi-bar display. On the first
// frame, N lines are emitted followed by a newline; on subsequent frames
// the cursor is moved up N lines via CSI `F` (cursor-previous-line) so the
// same N rows are overwritten in place. \033[K clears each line to the
// right of the cursor before redraw so shrinking output doesn't leave
// stale tail characters.
func renderFrame(w io.Writer, lines []string, first bool) {
	if !first {
		fmt.Fprintf(w, "\033[%dF", len(lines))
	}
	for _, line := range lines {
		fmt.Fprintf(w, "%s\033[K\n", line)
	}
}

// rateTracker computes a rolling-window rate from periodic count samples.
// observe pushes a new sample and returns the rate over the oldest-to-newest
// span in the window (0 until two samples land). Cheap: bounded ring, no
// allocations after construction.
type rateTracker struct {
	samples []rateSample
	cap     int
}

type rateSample struct {
	t time.Time
	n int64
}

func newRateTracker(window int) *rateTracker {
	return &rateTracker{cap: window}
}

func (r *rateTracker) observe(t time.Time, n int64) float64 {
	r.samples = append(r.samples, rateSample{t: t, n: n})
	if len(r.samples) > r.cap {
		r.samples = r.samples[len(r.samples)-r.cap:]
	}
	if len(r.samples) < 2 {
		return 0
	}
	oldest := r.samples[0]
	dt := t.Sub(oldest.t).Seconds()
	if dt <= 0 {
		return 0
	}
	return float64(n-oldest.n) / dt
}

const progressBarWidth = 30

// renderBarLine formats a single progress bar row. Bars share a fixed
// column layout so multi-bar output stacks cleanly: label is padded to a
// stable width, the bar is fixed at progressBarWidth, and the rate is
// formatted with a stable-width k/M suffix.
func renderBarLine(label string, n, total int64, elapsed time.Duration, rate float64) string {
	pct := 0
	filled := 0
	if total > 0 {
		pct = int(100 * n / total)
		if pct > 100 {
			pct = 100
		}
		filled = int(int64(progressBarWidth) * n / total)
		if filled > progressBarWidth {
			filled = progressBarWidth
		}
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", progressBarWidth-filled)
	return fmt.Sprintf("%-10s [%s] %3d%% (%d/%d) %.1fs (%s/s)",
		label, bar, pct, n, total, elapsed.Seconds(), formatRate(rate))
}

// formatRate prints msg/s with a k/M suffix when the number is large enough
// to be unwieldy in the progress line. Keeps the line width stable across
// orders of magnitude — important since the bar is a carriage-return
// overwrite, not a scrolled line.
func formatRate(r float64) string {
	switch {
	case r >= 1_000_000:
		return fmt.Sprintf("%.2fM", r/1_000_000)
	case r >= 10_000:
		return fmt.Sprintf("%.1fk", r/1_000)
	case r >= 1_000:
		return fmt.Sprintf("%.2fk", r/1_000)
	default:
		return fmt.Sprintf("%.0f", r)
	}
}
