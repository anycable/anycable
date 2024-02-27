package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
)

func BenchmarkTracer(b *testing.B) {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})
	tracer := NewTracer(handler)

	configs := []struct {
		tracer *Tracer
		active bool
	}{
		{tracer, true},
		{tracer, false},
		{nil, false},
	}

	for _, config := range configs {
		b.Run(fmt.Sprintf("tracer=%t active=%t", config.tracer != nil, config.active), func(b *testing.B) {
			var h slog.Handler = handler

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
				logger.Debug("test")
			}
		})
	}
}
