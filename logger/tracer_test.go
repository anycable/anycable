package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	_ "unsafe"
)

//go:linkname IgnorePC log/slog/internal.IgnorePC
var IgnorePC = true

func BenchmarkTracer(b *testing.B) {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})
	tracer := NewTracer(handler)

	configs := []struct {
		tracer   *Tracer
		active   bool
		ignorePC bool
	}{
		{tracer, true, false},
		{tracer, true, true},
		{tracer, false, false},
		{tracer, false, true},
		{nil, false, false},
		{nil, false, true},
	}

	for _, config := range configs {
		b.Run(fmt.Sprintf("tracer=%t active=%t ignorePC=%t", config.tracer != nil, config.active, config.ignorePC), func(b *testing.B) {
			var h slog.Handler = handler

			IgnorePC = config.ignorePC

			if config.tracer != nil {
				tracer := config.tracer
				go tracer.Run(func(msg string) {})
				defer tracer.Shutdown(context.Background())

				if config.active {
					tracer.Subscribe()
					defer tracer.Unsubscribe()
				}

				h = tracer
			}

			logger := slog.New(h)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				logger.Debug("test", "key", 1, "key2", "value2", "key3", 3.14)
			}
		})
	}
}
