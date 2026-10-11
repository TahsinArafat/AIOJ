package generate

import (
	"github.com/tahsinarafat/aioj/internal/model"
	"testing"
)

func TestValidateProposalFullPackageRequiresGeneratorAndCuratedCases(t *testing.T) {
	p := &Proposal{Title: "A", Description: "D", InputFormat: "I", OutputFormat: "O", SampleCases: sample(), SolutionCode: "int main(){}", SolutionLanguage: "cpp-gpp-64", EditorialTitle: "E", EditorialContent: "C"}
	report := ValidateProposal(p, "full_problem")
	if report.Valid {
		t.Fatal("proposal without generator/cases was accepted")
	}
	p.GeneratorCode = "int main(){}"
	p.GeneratorLanguage = "cpp-gpp-64"
	p.Seeds = []int64{1}
	p.CuratedCases = []CuratedCase{{Name: "edge-1", Input: "1\n"}}
	if got := ValidateProposal(p, "full_problem"); !got.Valid {
		t.Fatalf("complete proposal rejected: %+v", got.Checks)
	}
}

func TestValidateProposalRejectsUnsafeOrDuplicateCaseNames(t *testing.T) {
	p := &Proposal{SolutionCode: "x", SolutionLanguage: "cpp-gpp-64", GeneratorCode: "x", GeneratorLanguage: "cpp-gpp-64", Seeds: []int64{1}, CuratedCases: []CuratedCase{{Name: "../evil", Input: "1"}, {Name: "../evil", Input: "2"}}}
	if ValidateProposal(p, "tests").Valid {
		t.Fatal("unsafe testcase names accepted")
	}
}

func TestParseAndValidateProposalRejectsUnknownFields(t *testing.T) {
	if _, _, err := ParseAndValidateProposal(`{"solution_code":"x","solution_language":"cpp","unexpected":true}`, "solution"); err == nil {
		t.Fatal("unknown JSON field accepted")
	}
}

func sample() []model.SampleCase { return []model.SampleCase{{Input: "1\n", Output: "1\n"}} }
