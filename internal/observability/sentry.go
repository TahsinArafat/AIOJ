package observability

import (
	"log/slog"
	"os"

	"github.com/getsentry/sentry-go"
)

// InitSentry initializes the global Sentry client if SENTRY_DSN is set.
// Returns a shutdown func that flushes pending events.
func InitSentry() func() {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		slog.Info("sentry: DSN not set, skipping initialization")
		return func() {}
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		TracesSampleRate: 0.1,
		Environment:      os.Getenv("SENTRY_ENVIRONMENT"),
		Release:          os.Getenv("AIOJ_RELEASE"),
	})
	if err != nil {
		slog.Error("sentry init failed", "err", err)
		return func() {}
	}
	slog.Info("sentry: initialized", "env", os.Getenv("SENTRY_ENVIRONMENT"))
	return func() { sentry.Flush(2) }
}
