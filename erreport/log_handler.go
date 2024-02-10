package erreport

import (
	"context"
	"log/slog"
)

type LogHandler struct {
	slog.Handler
	reporter Reporter
}

const (
	shortErrKey = "err"
	longErrKey  = "error"
)

var _ slog.Handler = (*LogHandler)(nil)

func NewLogHandler(handler slog.Handler, reporter Reporter) *LogHandler {
	return &LogHandler{Handler: handler, reporter: reporter}
}

func (h *LogHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Level == slog.LevelError {
		record.Attrs(func(attr slog.Attr) bool {
			if attr.Key == shortErrKey || attr.Key == longErrKey {
				if err, ok := attr.Value.Any().(error); ok {
					h.reporter.CaptureException(err) // nolint: errcheck
				}
			}

			return true
		})
	}

	return h.Handler.Handle(ctx, record)
}

// Enabled returns true if the handler is enabled for the given level
func (h *LogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

// WithAttrs returns a new LogHandler with attributes
func (h *LogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return NewLogHandler(h.Handler.WithAttrs(attrs), h.reporter)
}

// WithGroup returns a new LogHandle with group
func (h *LogHandler) WithGroup(name string) slog.Handler {
	return NewLogHandler(h.Handler.WithGroup(name), h.reporter)
}
