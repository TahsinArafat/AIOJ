package postgres

import (
	"testing"
	"time"
)

// The feed cursor is what stops a polling scorer from re-reading or skipping a
// run. It is a (created_at, id) pair because created_at alone is not unique —
// two submissions can share a timestamp, and an order-dependent tie would make
// one of them invisible forever.
func TestSplitFeedCursor(t *testing.T) {
	at := time.Date(2026, 3, 4, 5, 6, 7, 123456789, time.UTC)
	valid := at.Format(time.RFC3339Nano) + "|11111111-1111-1111-1111-111111111111"

	gotAt, gotID, ok := splitFeedCursor(valid)
	if !ok {
		t.Fatal("a cursor this package produced was rejected")
	}
	if !gotAt.Equal(at) {
		t.Fatalf("time = %v, want %v", gotAt, at)
	}
	if gotID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("id = %q", gotID)
	}

	bad := []string{"", "|", "abc|def", "no-separator", "2026-03-04T05:06:07Z|"}
	for _, c := range bad {
		if _, _, ok := splitFeedCursor(c); ok {
			t.Fatalf("splitFeedCursor(%q) accepted a malformed cursor", c)
		}
	}
}

// A round-trip: whatever the list returns must be accepted by the next call.
func TestFeedCursorRoundTrip(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Nanosecond)
	next := at.Format(time.RFC3339Nano) + "|22222222-2222-2222-2222-222222222222"

	gotAt, _, ok := splitFeedCursor(next)
	if !ok || !gotAt.Equal(at) {
		t.Fatalf("round trip failed: ok=%v got=%v want=%v", ok, gotAt, at)
	}
}
