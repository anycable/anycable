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

// startProgress kicks off a 100ms ticker that rewrites a single line on
// stderr with `[bar] pct% (n/total)`. getCount is sampled each tick; total
// is the schedule's deterministic max. Returns a no-op handle in
// non-interactive mode.
func (u *ui) startProgress(label string, total int64, getCount func() int64) *progress {
	p := &progress{done: make(chan struct{}), exit: make(chan struct{})}
	if !u.interactive {
		close(p.exit)
		return p
	}
	go func() {
		defer close(p.exit)
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		start := time.Now()
		for {
			select {
			case <-t.C:
				renderBar(u.out, label, getCount(), total, time.Since(start))
			case <-p.done:
				renderBar(u.out, label, getCount(), total, time.Since(start))
				fmt.Fprintln(u.out)
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

const progressBarWidth = 30

func renderBar(w io.Writer, label string, n, total int64, elapsed time.Duration) {
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
	fmt.Fprintf(w, "\r%s [%s] %3d%% (%d/%d) %.1fs", label, bar, pct, n, total, elapsed.Seconds())
}
