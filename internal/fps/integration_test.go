package fps

import (
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

func TestImportExportRoundtrip(t *testing.T) {
	original := &model.Problem{
		ID:          "round-trip-test",
		Title:       "Sum Loop",
		TimeLimit:   2000,
		MemoryLimit: 131072, // 128MB in KB
		Description: "Given two integers A and B, print their sum.",
		InputFormat: "Two integers on one line.",
		OutputFormat: "A single integer — their sum.",
		Hint:        "Use addition.",
		Source:      "AIOJ 2026",
		Tags:        []string{"math", "easy"},
		SampleCases: []model.SampleCase{
			{Input: "3 4", Output: "7"},
			{Input: "100 200", Output: "300"},
		},
		Visible: true,
	}

	// Step 1: Generate XML from problem
	xmlBytes, err := GenerateXML([]*model.Problem{original})
	if err != nil {
		t.Fatalf("GenerateXML failed: %v", err)
	}

	if len(xmlBytes) == 0 {
		t.Fatal("GenerateXML returned empty bytes")
	}

	// Step 2: Parse XML back into problems
	parsed, err := ParseXML(xmlBytes)
	if err != nil {
		t.Fatalf("ParseXML failed: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 problem, got %d", len(parsed))
	}

	got := parsed[0]

	// Step 3: Compare key fields
	if got.Title != original.Title {
		t.Errorf("Title: want %q, got %q", original.Title, got.Title)
	}
	if got.TimeLimit != original.TimeLimit {
		t.Errorf("TimeLimit: want %d, got %d", original.TimeLimit, got.TimeLimit)
	}
	if got.MemoryLimit != original.MemoryLimit {
		t.Errorf("MemoryLimit (in KB): want %d, got %d", original.MemoryLimit, got.MemoryLimit)
	}
	if got.Description != original.Description {
		t.Errorf("Description: want %q, got %q", original.Description, got.Description)
	}
	if got.InputFormat != original.InputFormat {
		t.Errorf("InputFormat: want %q, got %q", original.InputFormat, got.InputFormat)
	}
	if got.OutputFormat != original.OutputFormat {
		t.Errorf("OutputFormat: want %q, got %q", original.OutputFormat, got.OutputFormat)
	}
	if got.Hint != original.Hint {
		t.Errorf("Hint: want %q, got %q", original.Hint, got.Hint)
	}
	if got.Source != original.Source {
		t.Errorf("Source: want %q, got %q", original.Source, got.Source)
	}
	if len(got.Tags) != len(original.Tags) {
		t.Errorf("Tags length: want %d, got %d", len(original.Tags), len(got.Tags))
	} else {
		for i, tag := range original.Tags {
			if got.Tags[i] != tag {
				t.Errorf("Tags[%d]: want %q, got %q", i, tag, got.Tags[i])
			}
		}
	}
	if len(got.SampleCases) != len(original.SampleCases) {
		t.Errorf("SampleCases length: want %d, got %d", len(original.SampleCases), len(got.SampleCases))
	} else {
		for i, sc := range original.SampleCases {
			if got.SampleCases[i].Input != sc.Input {
				t.Errorf("SampleCases[%d].Input: want %q, got %q", i, sc.Input, got.SampleCases[i].Input)
			}
			if got.SampleCases[i].Output != sc.Output {
				t.Errorf("SampleCases[%d].Output: want %q, got %q", i, sc.Output, got.SampleCases[i].Output)
			}
		}
	}
}

func TestImportExportRoundtrip_WithSPJ(t *testing.T) {
	original := &model.Problem{
		ID:            "spj-test",
		Title:         "Checker Problem",
		TimeLimit:     1000,
		MemoryLimit:   262144,
		Description:   "Check multiple valid outputs",
		SPJ:           true,
		SPJLanguage:   "cpp",
		SPJSourceCode: "#include <iostream>\nint main() { return 0; }",
	}

	xmlBytes, err := GenerateXML([]*model.Problem{original})
	if err != nil {
		t.Fatalf("GenerateXML failed: %v", err)
	}

	parsed, err := ParseXML(xmlBytes)
	if err != nil {
		t.Fatalf("ParseXML failed: %v", err)
	}

	if len(parsed) != 1 {
		t.Fatalf("expected 1 problem, got %d", len(parsed))
	}

	got := parsed[0]
	if !got.SPJ {
		t.Error("SPJ should be true after round-trip")
	}
	if got.SPJLanguage != "cpp" {
		t.Errorf("SPJLanguage: want %q, got %q", "cpp", got.SPJLanguage)
	}
	if !strings.Contains(got.SPJSourceCode, "#include") {
		t.Errorf("SPJSourceCode content lost: got %q", got.SPJSourceCode)
	}
}

func TestImportExportRoundtrip_MultipleProblems(t *testing.T) {
	problems := []*model.Problem{
		{
			Title:       "Problem A",
			TimeLimit:   1000,
			MemoryLimit: 65536,
			Description: "First problem",
			Tags:        []string{"easy"},
		},
		{
			Title:       "Problem B",
			TimeLimit:   2000,
			MemoryLimit: 131072,
			Description: "Second problem",
			Tags:        []string{"hard"},
		},
		{
			Title:       "Problem C",
			TimeLimit:   3000,
			MemoryLimit: 262144,
			Description: "Third problem",
		},
	}

	xmlBytes, err := GenerateXML(problems)
	if err != nil {
		t.Fatalf("GenerateXML failed: %v", err)
	}

	parsed, err := ParseXML(xmlBytes)
	if err != nil {
		t.Fatalf("ParseXML failed: %v", err)
	}

	if len(parsed) != 3 {
		t.Fatalf("expected 3 problems, got %d", len(parsed))
	}

	for i, orig := range problems {
		got := parsed[i]
		if got.Title != orig.Title {
			t.Errorf("problem[%d] Title: want %q, got %q", i, orig.Title, got.Title)
		}
		if got.TimeLimit != orig.TimeLimit {
			t.Errorf("problem[%d] TimeLimit: want %d, got %d", i, orig.TimeLimit, got.TimeLimit)
		}
		if got.MemoryLimit != orig.MemoryLimit {
			t.Errorf("problem[%d] MemoryLimit: want %d, got %d", i, orig.MemoryLimit, got.MemoryLimit)
		}
	}
}
