package model

import "testing"

func TestGetColor(t *testing.T) {
	tests := []struct {
		name     string
		rating   int
		expected string
	}{
		// Boundary edges
		{name: "legendary grandmaster high-end", rating: 3500, expected: ColorLegendaryGrandmaster},
		{name: "legendary grandmaster threshold", rating: 2900, expected: ColorLegendaryGrandmaster},
		{name: "international grandmaster low-end", rating: 2899, expected: ColorInternationalGrandmaster},
		{name: "international grandmaster threshold", rating: 2600, expected: ColorInternationalGrandmaster},
		{name: "grandmaster low-end", rating: 2599, expected: ColorGrandmaster},
		{name: "grandmaster threshold", rating: 2400, expected: ColorGrandmaster},
		{name: "international master low-end", rating: 2399, expected: ColorInternationalMaster},
		{name: "international master threshold", rating: 2300, expected: ColorInternationalMaster},
		{name: "master low-end", rating: 2299, expected: ColorMaster},
		{name: "master threshold", rating: 2100, expected: ColorMaster},
		{name: "candidate master low-end", rating: 2099, expected: ColorCandidateMaster},
		{name: "candidate master threshold", rating: 1900, expected: ColorCandidateMaster},
		{name: "expert low-end", rating: 1899, expected: ColorExpert},
		{name: "expert threshold", rating: 1600, expected: ColorExpert},
		{name: "specialist low-end", rating: 1599, expected: ColorSpecialist},
		{name: "specialist threshold", rating: 1400, expected: ColorSpecialist},
		{name: "pupil low-end", rating: 1399, expected: ColorPupil},
		{name: "pupil threshold", rating: 1200, expected: ColorPupil},
		{name: "newbie low-end", rating: 1199, expected: ColorNewbie},
		// Extreme values
		{name: "zero rating", rating: 0, expected: ColorNewbie},
		{name: "negative rating", rating: -100, expected: ColorNewbie},
		{name: "typical newbie", rating: 500, expected: ColorNewbie},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetColor(tt.rating)
			if got != tt.expected {
				t.Errorf("GetColor(%d) = %q, want %q", tt.rating, got, tt.expected)
			}
		})
	}
}

func TestGetColorHex(t *testing.T) {
	tests := []struct {
		name     string
		rating   int
		expected string
	}{
		{name: "legendary grandmaster", rating: 3000, expected: "#FF0000"},
		{name: "grandmaster", rating: 2500, expected: "#FF8C00"},
		{name: "master", rating: 2200, expected: "#FFD700"},
		{name: "candidate master", rating: 2000, expected: "#AA00AA"},
		{name: "expert", rating: 1700, expected: "#0000FF"},
		{name: "specialist", rating: 1500, expected: "#03A89E"},
		{name: "pupil", rating: 1300, expected: "#008000"},
		{name: "newbie", rating: 1000, expected: "#808080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetColorHex(tt.rating)
			if got != tt.expected {
				t.Errorf("GetColorHex(%d) = %q, want %q", tt.rating, got, tt.expected)
			}
		})
	}
}

func TestGetDivisionRange(t *testing.T) {
	tests := []struct {
		name     string
		division int
		wantMin  int
		wantMax  int
	}{
		{name: "division 1", division: Division1, wantMin: 1900, wantMax: 9999},
		{name: "division 2", division: Division2, wantMin: 0, wantMax: 2099},
		{name: "division 3", division: Division3, wantMin: 0, wantMax: 1599},
		{name: "division 4", division: Division4, wantMin: 0, wantMax: 1399},
		{name: "division none", division: DivisionNone, wantMin: 0, wantMax: 9999},
		{name: "invalid division", division: 99, wantMin: 0, wantMax: 9999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMin, gotMax := GetDivisionRange(tt.division)
			if gotMin != tt.wantMin || gotMax != tt.wantMax {
				t.Errorf("GetDivisionRange(%d) = (%d, %d), want (%d, %d)",
					tt.division, gotMin, gotMax, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestIsEligibleForDivision(t *testing.T) {
	tests := []struct {
		name     string
		division int
		rating   int
		expected bool
	}{
		{name: "div1 eligible at 1900", division: Division1, rating: 1900, expected: true},
		{name: "div1 eligible at 2500", division: Division1, rating: 2500, expected: true},
		{name: "div1 ineligible at 1899", division: Division1, rating: 1899, expected: false},
		{name: "div2 eligible at 0", division: Division2, rating: 0, expected: true},
		{name: "div2 eligible at 2099", division: Division2, rating: 2099, expected: true},
		{name: "div2 ineligible at 2100", division: Division2, rating: 2100, expected: false},
		{name: "div3 eligible at 1500", division: Division3, rating: 1500, expected: true},
		{name: "div3 ineligible at 1600", division: Division3, rating: 1600, expected: false},
		{name: "div4 eligible at 1300", division: Division4, rating: 1300, expected: true},
		{name: "div4 ineligible at 1400", division: Division4, rating: 1400, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsEligibleForDivision(tt.division, tt.rating)
			if got != tt.expected {
				t.Errorf("IsEligibleForDivision(%d, %d) = %v, want %v",
					tt.division, tt.rating, got, tt.expected)
			}
		})
	}
}

func TestDefaultEducationalConfig(t *testing.T) {
	cfg := DefaultEducationalConfig()

	if cfg.HackPhaseHours != 24 {
		t.Errorf("HackPhaseHours = %d, want 24", cfg.HackPhaseHours)
	}
	if !cfg.ShowSolutions {
		t.Error("ShowSolutions should be true")
	}
	if !cfg.AllowUpsolving {
		t.Error("AllowUpsolving should be true")
	}
	if len(cfg.RatedForDivisions) != 2 {
		t.Fatalf("RatedForDivisions length = %d, want 2", len(cfg.RatedForDivisions))
	}
	if cfg.RatedForDivisions[0] != 2 || cfg.RatedForDivisions[1] != 3 {
		t.Errorf("RatedForDivisions = %v, want [2, 3]", cfg.RatedForDivisions)
	}
}

func TestProblemHasSubtasks(t *testing.T) {
	p := &Problem{
		TestCaseScore: []TestCaseScore{
			{InputName: "1.in", OutputName: "1.out", Score: 10, SubtaskID: 1},
			{InputName: "2.in", OutputName: "2.out", Score: 10, SubtaskID: 1},
			{InputName: "3.in", OutputName: "3.out", Score: 20, SubtaskID: 2},
		},
	}

	if !p.HasSubtasks() {
		t.Error("expected HasSubtasks() = true")
	}

	subtasks := p.GetSubtasks()
	if len(subtasks) != 2 {
		t.Errorf("expected 2 subtasks, got %d", len(subtasks))
	}
	if len(subtasks[1]) != 2 {
		t.Errorf("expected 2 cases in subtask 1, got %d", len(subtasks[1]))
	}
	if len(subtasks[2]) != 1 {
		t.Errorf("expected 1 case in subtask 2, got %d", len(subtasks[2]))
	}
}

func TestProblemNoSubtasks(t *testing.T) {
	p := &Problem{
		TestCaseScore: []TestCaseScore{
			{InputName: "1.in", OutputName: "1.out", Score: 10},
			{InputName: "2.in", OutputName: "2.out", Score: 10},
		},
	}

	if p.HasSubtasks() {
		t.Error("expected HasSubtasks() = false")
	}

	subtasks := p.GetSubtasks()
	if len(subtasks) != 0 {
		t.Errorf("expected 0 subtasks, got %d", len(subtasks))
	}
}

func TestDivisionNames(t *testing.T) {
	tests := []struct {
		division int
		expected string
	}{
		{DivisionNone, "Open"},
		{Division1, "Div. 1"},
		{Division2, "Div. 2"},
		{Division3, "Div. 3"},
		{Division4, "Div. 4"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := DivisionNames[tt.division]
			if got != tt.expected {
				t.Errorf("DivisionNames[%d] = %q, want %q", tt.division, got, tt.expected)
			}
		})
	}
}

func TestRoleConstants(t *testing.T) {
	// These must match the DB CHECK constraint in 000001_init.up.sql
	roles := []string{RoleAdmin, RoleTeacher, RoleUser, RoleBot}
	for _, r := range roles {
		if r == "" {
			t.Errorf("role constant is empty string")
		}
	}
	if RoleAdmin != "admin" {
		t.Errorf("RoleAdmin = %q, want %q", RoleAdmin, "admin")
	}
}

func TestAccessLevelConstants(t *testing.T) {
	levels := []string{AccessLevelOwner, AccessLevelCoAuthor, AccessLevelManager, AccessLevelJudge, AccessLevelTester}
	seen := map[string]bool{}
	for _, l := range levels {
		if l == "" {
			t.Errorf("access level constant is empty string")
		}
		if seen[l] {
			t.Errorf("duplicate access level constant: %q", l)
		}
		seen[l] = true
	}
}
