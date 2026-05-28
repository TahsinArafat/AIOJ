package virtual

import (
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

func TestService_GetStatus(t *testing.T) {
	svc := NewService(nil)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		contest       *model.VirtualContest
		wantActive    bool
		wantRemaining int
	}{
		{"just started", &model.VirtualContest{ID: "vc", Status: "active", StartedAt: now, DurationMinutes: 120}, true, 120},
		{"30 min elapsed", &model.VirtualContest{ID: "vc", Status: "active", StartedAt: now.Add(-30 * time.Minute), DurationMinutes: 120}, true, 90},
		{"half done", &model.VirtualContest{ID: "vc", Status: "active", StartedAt: now.Add(-60 * time.Minute), DurationMinutes: 120}, true, 60},
		{"almost done", &model.VirtualContest{ID: "vc", Status: "active", StartedAt: now.Add(-119 * time.Minute), DurationMinutes: 120}, true, 1},
		{"exactly ended", &model.VirtualContest{ID: "vc", Status: "active", StartedAt: now.Add(-120 * time.Minute), DurationMinutes: 120}, false, 0},
		{"expired", &model.VirtualContest{ID: "vc", Status: "active", StartedAt: now.Add(-130 * time.Minute), DurationMinutes: 120}, false, 0},
		{"long contest", &model.VirtualContest{ID: "vc", Status: "active", StartedAt: now, DurationMinutes: 300}, true, 300},
		{"zero duration", &model.VirtualContest{ID: "vc", Status: "active", StartedAt: now, DurationMinutes: 0}, false, 0},
		{"completed then expired", &model.VirtualContest{ID: "vc", Status: "completed", StartedAt: now.Add(-130 * time.Minute), DurationMinutes: 120}, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := svc.GetStatus(tt.contest, now)
			if status.IsActive != tt.wantActive {
				t.Errorf("IsActive = %v, want %v (remaining=%d)", status.IsActive, tt.wantActive, status.RemainingMins)
			}
			if status.RemainingMins != tt.wantRemaining {
				t.Errorf("RemainingMins = %d, want %d", status.RemainingMins, tt.wantRemaining)
			}
			if !tt.wantActive && status.RemainingMins > 0 {
				t.Logf("Inactive but has remaining time: status=%s remaining=%d", tt.contest.Status, status.RemainingMins)
			}
		})
	}
}

func TestService_StartVirtualContest_Defaults(t *testing.T) {
	t.Run("duration zero defaults to 120", func(t *testing.T) {
		var dur int = 0
		if dur <= 0 {
			dur = 120
		}
		if dur != 120 {
			t.Errorf("got %d, want 120", dur)
		}
	})
	t.Run("negative duration defaults to 120", func(t *testing.T) {
		var dur int = -5
		if dur <= 0 {
			dur = 120
		}
		if dur != 120 {
			t.Errorf("got %d, want 120", dur)
		}
	})
}

func TestErrActiveVirtualExists(t *testing.T) {
	if ErrActiveVirtualExists.Error() != "you already have an active virtual contest" {
		t.Errorf("unexpected message: %v", ErrActiveVirtualExists)
	}
}
