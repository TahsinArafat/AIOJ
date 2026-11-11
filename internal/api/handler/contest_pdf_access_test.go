package handler

import (
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * The Booklet tab opens /api/contests/{id}/pdf with a plain <a href>, which cannot
 * carry an Authorization header. That worked only while the contest was public; on
 * a draft the guard correctly answers 404 and the link had no way to authenticate.
 * The reported "doesn't work" was the guard doing its job plus a UI that never sent
 * credentials.
 *
 * The downloadable-PDF decision is two independent gates:
 *   1. can the caller READ the contest at all (canReadContest), and
 *   2. is the booklet enabled, or is the caller staff (the isJudge override).
 * These pin gate 1 across the caller matrix so a future change cannot quietly
 * widen it back open.
 */

func pdfContest(visible bool) *model.Contest {
	now := time.Now()
	return &model.Contest{
		ID: "c1", Title: "Round", Visible: visible,
		StartTime: now.Add(-2 * time.Hour), EndTime: now.Add(time.Hour),
		PDFEnabled: true, CreatedBy: "owner1",
	}
}

func TestBookletRequiresAReadableContest(t *testing.T) {
	cases := []struct {
		name    string
		contest *model.Contest
		claims  *auth.Claims
		staff   bool
		want    bool
	}{
		// A published, started round is public — which is why the link appeared to
		// work until the contest became a draft.
		{"published and started, anonymous", pdfContest(true), nil, false, true},
		{"draft, anonymous", pdfContest(false), nil, false, false},
		{"draft, unrelated user", pdfContest(false), &auth.Claims{UserID: "u", Role: "user"}, false, false},
		{"draft, unrelated setter", pdfContest(false), &auth.Claims{UserID: "s2", Role: "setter"}, false, false},
		// Staff preparing the round must still get the booklet.
		{"draft, owner", pdfContest(false), &auth.Claims{UserID: "owner1", Role: "setter"}, false, true},
		{"draft, manager", pdfContest(false), &auth.Claims{UserID: "m", Role: "setter"}, true, true},
		{"draft, admin", pdfContest(false), &auth.Claims{UserID: "a", Role: "admin"}, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := canReadContest(t.Context(), contestAccessStub{access: tc.staff}, tc.contest, tc.claims)
			if got != tc.want {
				t.Fatalf("canReadContest = %v, want %v", got, tc.want)
			}
		})
	}
}

// The disabled-booklet override is staff-only; a plain user must not bypass it on
// a round they can otherwise read.
func TestBookletDisabledOverrideIsStaffOnly(t *testing.T) {
	canOverride := func(claims *auth.Claims, staff bool) bool {
		return claims != nil && (claims.Role == "admin" || staff)
	}

	if canOverride(&auth.Claims{UserID: "u", Role: "user"}, false) {
		t.Fatal("a plain user can override a disabled booklet")
	}
	if canOverride(nil, false) {
		t.Fatal("an anonymous caller can override a disabled booklet")
	}
	if !canOverride(&auth.Claims{UserID: "a", Role: "admin"}, false) {
		t.Fatal("an admin cannot override a disabled booklet")
	}
}
