package logger

import (
	"bytes"
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

const (
	maxBufSize       = 256 * 1024 // 256KB
	minFlushInterval = 250 * time.Millisecond
)

type TracerOutput func(msg string)

type TraceCommand int

const (
	TraceCommandRecord TraceCommand = iota
	TraceCommandFlush
	TraceCommandStop
)

type TraceEntry struct {
	record *slog.Record
	// printer keeps the reference to the current printer
	// to carry on log attributes and groups
	printer slog.Handler
	cmd     TraceCommand
}

type Tracer struct {
	parent slog.Handler
	output TracerOutput

	active *atomic.Int64
	ch     chan *TraceEntry
	timer  *time.Timer
	buf    *bytes.Buffer

	// A log handler we use to format records
	printer slog.Handler

	// Internal logger
	log *slog.Logger
}

var _ slog.Handler = (*Tracer)(nil)

func NewTracer(parent slog.Handler) *Tracer {
	buf := &bytes.Buffer{}
	return &Tracer{
		parent:  parent,
		ch:      make(chan *TraceEntry, 2048),
		buf:     buf,
		active:  &atomic.Int64{},
		log:     slog.New(parent).With("context", "tracer"),
		printer: slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}),
	}
}

func (t *Tracer) Enabled(ctx context.Context, level slog.Level) bool {
	if t.active.Load() == 0 {
		return t.parent.Enabled(ctx, level)
	}

	return true
}

func (t *Tracer) Handle(ctx context.Context, r slog.Record) error {
	if t.active.Load() == 0 {
		return t.parent.Handle(ctx, r)
	}

	t.enqueueRecord(&r)

	if t.parent.Enabled(ctx, r.Level) {
		return t.parent.Handle(ctx, r)
	}

	return nil
}

func (t *Tracer) WithAttrs(attrs []slog.Attr) slog.Handler {
	tr := t.Clone()
	tr.parent = t.parent.WithAttrs(attrs)
	tr.printer = t.printer.WithAttrs(attrs)
	return tr
}

func (t *Tracer) WithGroup(name string) slog.Handler {
	tr := t.Clone()
	tr.parent = t.parent.WithGroup(name)
	tr.printer = t.printer.WithGroup(name)
	return tr
}

// Run starts a Go routine which published log messages in the background
func (t *Tracer) Run(out TracerOutput) {
	t.log.Info("starting log tracer")

	t.output = out

	for entry := range t.ch {
		if entry.cmd == TraceCommandStop {
			if t.timer != nil {
				t.timer.Stop()
			}
			return
		}

		if entry.cmd == TraceCommandFlush {
			t.flush()
			continue
		}

		entry.printer.Handle(context.Background(), *entry.record) // nolint: errcheck

		if t.buf.Len() > maxBufSize {
			t.flush()
		} else {
			t.resetTimer()
		}
	}
}

func (t *Tracer) Shutdown(ctx context.Context) {
	t.ch <- &TraceEntry{cmd: TraceCommandStop}
}

func (t *Tracer) Subscribe() {
	t.active.Add(1)
}

func (t *Tracer) Unsubscribe() {
	t.active.Add(-1)
}

func (t *Tracer) Handler() slog.Handler {
	return t.parent
}

// Clone returns a new Tracer with the same parent handler and buffers
func (t *Tracer) Clone() *Tracer {
	return &Tracer{
		parent: t.parent,
		output: t.output,
		active: t.active,
		ch:     t.ch,
		buf:    t.buf,
		log:    t.log,
	}
}

func (t *Tracer) enqueueRecord(r *slog.Record) {
	// Make sure we don't block the main thread; it's okay to ignore the record if the channel is full
	select {
	case t.ch <- &TraceEntry{record: r, cmd: TraceCommandRecord, printer: t.printer}:
	default:
	}
}

func (t *Tracer) resetTimer() {
	if t.timer != nil {
		t.timer.Stop()
	}
	t.timer = time.AfterFunc(minFlushInterval, t.sendFlush)
}

func (t *Tracer) sendFlush() {
	t.ch <- &TraceEntry{cmd: TraceCommandFlush}
}

func (t *Tracer) flush() {
	if t.buf.Len() == 0 {
		return
	}

	msg := t.buf.String()

	t.output(msg)

	t.buf.Reset()
}
