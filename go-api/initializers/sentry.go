package initializers

import (
	"log/slog"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
)

// SetupSentry turns on error reporting when SENTRY_DSN is set; without it nothing is sent and the
// returned function does nothing. Call the returned function before the process exits so queued
// events are delivered.
//
//	SENTRY_DSN          where to send errors (from your Sentry project)
//	SENTRY_ENVIRONMENT  e.g. production or staging (default: production)
//	SENTRY_RELEASE      optional version label, such as a git commit
//
// What is reported is deliberately narrow: panics, with their stack trace and the request id.
// Request bodies, headers, cookies and user details are never attached, because they can hold
// documents, passwords and session tokens.
func SetupSentry(logger *slog.Logger) (flush func()) {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		return func() {}
	}
	environment := os.Getenv("SENTRY_ENVIRONMENT")
	if environment == "" {
		environment = "production"
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      environment,
		Release:          os.Getenv("SENTRY_RELEASE"),
		ServerName:       "go-api",
		AttachStacktrace: true,
		// Nothing user-related is collected by default; ScrubEvent guarantees it for every event.
		BeforeSend: ScrubEvent,
	})
	if err != nil {
		// A bad DSN must not stop the API from starting; reporting is a convenience.
		logger.Error("Sentry is configured but could not start; errors will not be reported", "error", err)
		return func() {}
	}
	logger.Info("error reporting enabled", "provider", "sentry", "environment", environment)
	return func() { sentry.Flush(2 * time.Second) }
}

// ScrubEvent removes everything that could carry user data from an event before it is sent.
func ScrubEvent(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	event.Request = nil
	event.User = sentry.User{}
	event.Breadcrumbs = nil
	return event
}
