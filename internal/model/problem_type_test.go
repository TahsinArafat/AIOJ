package model

import "testing"

func TestResolveProblemType(t *testing.T) {
	str := func(s string) *string { return &s }
	bl := func(b bool) *bool { return &b }

	cases := []struct {
		name        string
		explicit    *string
		interactive *bool
		want        string
	}{
		// Omitted entirely: no opinion, so the caller keeps the stored type.
		// This is what the statement form sends, and reading it as "standard"
		// demoted every interactive and output_only problem on a typo fix.
		{"neither field", nil, nil, ""},

		{"ticking interactive always wins", nil, bl(true), ProblemTypeInteractive},
		{"explicit interactive", str(ProblemTypeInteractive), nil, ProblemTypeInteractive},
		{"explicit interactive with box on", str(ProblemTypeInteractive), bl(true), ProblemTypeInteractive},

		// An interactive problem whose box is unticked must be demoted, or the
		// judge and the enum disagree forever.
		{"explicit interactive with box off demotes", str(ProblemTypeInteractive), bl(false), ProblemTypeStandard},
		{"unticking alone demotes", nil, bl(false), ProblemTypeStandard},

		{"output_only survives an absent boolean", str(ProblemTypeOutputOnly), nil, ProblemTypeOutputOnly},
		{"output_only survives an explicit false", str(ProblemTypeOutputOnly), bl(false), ProblemTypeOutputOnly},
		{"ticking interactive overrides output_only", str(ProblemTypeOutputOnly), bl(true), ProblemTypeInteractive},

		{"explicit standard", str(ProblemTypeStandard), nil, ProblemTypeStandard},
		{"explicit standard with box off", str(ProblemTypeStandard), bl(false), ProblemTypeStandard},

		// An unrecognized explicit value is ignored rather than stored; the
		// handlers reject it as a 400 before reaching here.
		{"unknown explicit falls back to no opinion", str("nonsense"), nil, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveProblemType(tc.explicit, tc.interactive); got != tc.want {
				t.Errorf("ResolveProblemType(%v, %v) = %q, want %q",
					derefStr(tc.explicit), derefBool(tc.interactive), got, tc.want)
			}
		})
	}
}

func TestValidEnums(t *testing.T) {
	for _, s := range []string{ProblemTypeStandard, ProblemTypeInteractive, ProblemTypeOutputOnly} {
		if !ValidProblemType(s) {
			t.Errorf("ValidProblemType(%q) = false", s)
		}
	}
	if ValidProblemType("") || ValidProblemType("nonsense") {
		t.Error("ValidProblemType accepted an invalid value")
	}
	if !ValidScoringMode("complete") || !ValidScoringMode("partial") || ValidScoringMode("vibes") {
		t.Error("ValidScoringMode is wrong")
	}
	if !ValidSubtaskAggregation("min") || !ValidSubtaskAggregation("sum") || ValidSubtaskAggregation("avg") {
		t.Error("ValidSubtaskAggregation is wrong")
	}
}

func derefStr(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func derefBool(p *bool) string {
	if p == nil {
		return "<nil>"
	}
	if *p {
		return "true"
	}
	return "false"
}
