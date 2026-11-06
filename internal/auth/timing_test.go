package auth

import (
	"testing"
	"time"
)

// TestWastePasswordComparisonTimeIsSlow guards the login timing equalizer: if
// this becomes a no-op, an unknown username answers in microseconds while a real
// one takes a bcrypt round, which is a usable username oracle.
func TestWastePasswordComparisonTimeIsSlow(t *testing.T) {
	start := time.Now()
	WastePasswordComparisonTime("anything")
	elapsed := time.Since(start)

	// bcrypt at cost 12 is tens of milliseconds on any machine that can run this
	// suite; 5ms is a conservative lower bound that still fails a no-op.
	if elapsed < 5*time.Millisecond {
		t.Fatalf("took %v, expected a real bcrypt comparison", elapsed)
	}
}

func TestDummyHashIsUnguessable(t *testing.T) {
	if len(dummyHash) == 0 {
		t.Fatal("dummy hash was not generated")
	}
	// The hash covers random bytes, so no candidate password may satisfy it.
	for _, guess := range []string{"", "password", "admin", "aioj-timing-equalizer"} {
		if CheckPassword(guess, string(dummyHash)) {
			t.Fatalf("dummy hash validated %q", guess)
		}
	}
}
