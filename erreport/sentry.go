package erreport

import (
	"log/slog"

	"github.com/anycable/anycable-go/version"
	sentry "github.com/getsentry/sentry-go"
)

type SentryReporter struct {
	log *slog.Logger
}

func (r *SentryReporter) CaptureException(err error) error {
	eventId := sentry.CaptureException(err)
	r.log.Debug("Sentry event sent", "event_id", eventId)
	return nil
}

// NewSentryReporter creates a new SentryReporter instance
func NewSentryReporter(dsn string, l *slog.Logger) (Reporter, error) {
	err := sentry.Init(sentry.ClientOptions{
		Dsn:     dsn,
		Release: "anycable@" + version.Version(),
	})
	if err != nil {
		return nil, err
	}
	return &SentryReporter{log: l.With("context", "sentry")}, nil
}
