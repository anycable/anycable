package erreport

import (
	"log/slog"
	"os"
)

type Reporter interface {
	CaptureException(err error) error
}

func ConfigureLogHandler(handler slog.Handler) slog.Handler {
	if dsn, ok := os.LookupEnv("SENTRY_DSN"); ok {
		reporter, err := NewSentryReporter(dsn, slog.Default())
		if err != nil {
			slog.Error("failed to initialize Sentry reporter", "error", err)
			return nil
		}

		slog.Info("Sentry errors reporting enabled")
		return NewLogHandler(handler, reporter)
	}
	return nil
}
