package observability

import (
	"testing"
)

func TestInitSentry_NoDSNIsNoop(t *testing.T) {
	t.Setenv("SENTRY_DSN", "")
	shutdown := InitSentry()
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown")
	}
	shutdown()
}
