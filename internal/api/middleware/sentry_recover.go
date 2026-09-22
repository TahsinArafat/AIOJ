package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/getsentry/sentry-go"
)

// SentryRecover captures panics and reports them to Sentry, then re-panics
// so chi's Recoverer can still turn them into 500s.
func SentryRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
					hub.CaptureException(fmt.Errorf("panic: %v", rec))
					hub.CaptureMessage(string(debug.Stack()))
				} else {
					sentry.CaptureException(fmt.Errorf("panic: %v", rec))
					sentry.CaptureMessage(string(debug.Stack()))
				}
				panic(rec)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
