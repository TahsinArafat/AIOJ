package generate

import (
	"strings"
	"testing"
)

// baseRequest returns a valid request that each test mutates to isolate one rule.
func baseRequest() Request {
	return Request{
		Tags:          []string{"dp"},
		Rating:        1600,
		TestCaseCount: 10,
	}
}

func TestNormalizeAcceptsMinimalRequestAndFillsDefaults(t *testing.T) {
	got, err := baseRequest().Normalize()
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	if got.ProblemType != "standard" {
		t.Errorf("ProblemType = %q, want standard", got.ProblemType)
	}
	if got.SolverMode != SolverSolo {
		t.Errorf("SolverMode = %q, want %q", got.SolverMode, SolverSolo)
	}
	if got.EventContext != EventPractice {
		t.Errorf("EventContext = %q, want %q", got.EventContext, EventPractice)
	}
	if got.FormatProfile != ProfileGeneral {
		t.Errorf("FormatProfile = %q, want %q", got.FormatProfile, ProfileGeneral)
	}
	// Rating tolerance defaults to target +/- 100.
	if got.RatingMin != 1500 || got.RatingMax != 1700 {
		t.Errorf("rating range = [%d,%d], want [1500,1700]", got.RatingMin, got.RatingMax)
	}
}

func TestNormalizeRatingRangeMustContainTarget(t *testing.T) {
	cases := []struct {
		name     string
		min, max int
		wantErr  bool
	}{
		{"target below range", 1900, 2100, true},
		{"target above range", 1200, 1400, true},
		{"range below floor", 700, 900, true},
		{"range above ceiling", 3400, 3600, true},
		{"target inside range", 1500, 1700, false},
		{"target equals bounds", 1600, 1600, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := baseRequest()
			req.RatingMin, req.RatingMax = tc.min, tc.max
			_, err := req.Normalize()
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for range [%d,%d] around target 1600", tc.min, tc.max)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNormalizeICPCProfileRejectsSubtasks(t *testing.T) {
	req := baseRequest()
	req.FormatProfile = ProfileICPC
	req.HasSubtasks = true

	if _, err := req.Normalize(); err == nil {
		t.Fatal("expected ICPC + subtasks to be rejected")
	} else if !strings.Contains(err.Error(), "ICPC") {
		t.Errorf("error = %q, want it to mention ICPC", err.Error())
	}

	// Same profile without subtasks is fine.
	req.HasSubtasks = false
	if _, err := req.Normalize(); err != nil {
		t.Fatalf("ICPC without subtasks should be valid: %v", err)
	}
}

func TestNormalizeICPCProfileForcesOnsiteTeamDefaults(t *testing.T) {
	req := baseRequest()
	req.FormatProfile = ProfileICPC
	req.SolverMode = SolverSolo
	req.EventContext = EventOnline
	got, err := req.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if got.SolverMode != SolverTeam || got.TeamSize != 3 || got.EventContext != EventOnsite || got.ExpectedSolveMinutes != 60 {
		t.Fatalf("ICPC normalization = %+v", got)
	}
}

func TestNormalizeRejectsInteractiveAndOutputOnly(t *testing.T) {
	for _, kind := range []string{"interactive", "output_only"} {
		req := baseRequest()
		req.ProblemType = kind
		if _, err := req.Normalize(); err == nil {
			t.Errorf("accepted %s", kind)
		}
	}
}

func TestNormalizeTeamDefaultsToThree(t *testing.T) {
	req := baseRequest()
	req.SolverMode = SolverTeam

	got, err := req.Normalize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TeamSize != 3 {
		t.Errorf("TeamSize = %d, want 3", got.TeamSize)
	}

	// An explicit size is preserved.
	req.TeamSize = 4
	got, err = req.Normalize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TeamSize != 4 {
		t.Errorf("TeamSize = %d, want 4", got.TeamSize)
	}
}

func TestNormalizeRejectsInvalidEnums(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Request)
	}{
		{"bad solver mode", func(r *Request) { r.SolverMode = "duo" }},
		{"bad event context", func(r *Request) { r.EventContext = "hybrid" }},
		{"bad format profile", func(r *Request) { r.FormatProfile = "topcoder" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := baseRequest()
			tc.mutate(&req)
			if _, err := req.Normalize(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestNormalizeRejectsOutOfBoundsNumbers(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Request)
	}{
		{"no tags", func(r *Request) { r.Tags = nil }},
		{"rating too low", func(r *Request) { r.Rating = 799 }},
		{"rating too high", func(r *Request) { r.Rating = 3501 }},
		{"zero test cases", func(r *Request) { r.TestCaseCount = 0 }},
		{"too many test cases", func(r *Request) { r.TestCaseCount = 101 }},
		{"team size too large", func(r *Request) { r.SolverMode = SolverTeam; r.TeamSize = 9 }},
		{"solve minutes too large", func(r *Request) { r.ExpectedSolveMinutes = 361 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := baseRequest()
			tc.mutate(&req)
			if _, err := req.Normalize(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
