package generate

import (
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

// Normalize is artifact-aware: which inputs are required and which are validated
// depends on the artifact. Before the split it enforced the full ruleset on
// every request, so an editorial job had to carry a topic, a rating and a
// test-case count that the run then ignored.
func TestNormalizeOnlyRequiresTopicsOfTheArtifactThatInventsAProblem(t *testing.T) {
	// No topics at all. Only the full package needs them.
	noTopics := Request{Rating: 1400, TestCaseCount: 10}

	if _, err := noTopics.Normalize(string(model.ArtifactFullProblem)); err == nil {
		t.Error("the full package must require topics: without them there is nothing to build around")
	}
	for _, a := range []model.ArtifactType{
		model.ArtifactStatement, model.ArtifactTests, model.ArtifactSolution, model.ArtifactEditorial,
	} {
		if _, err := noTopics.Normalize(string(a)); err != nil {
			t.Errorf("%s should not require topics: it works from the existing problem, got %v", a, err)
		}
	}
}

// A rating out of range is only a problem for the artifact that has a rating.
func TestNormalizeValidatesTheRatingOnlyWhereItIsUsed(t *testing.T) {
	bad := Request{Tags: []string{"dp"}, Rating: 99, TestCaseCount: 10}

	if _, err := bad.Normalize(string(model.ArtifactFullProblem)); err == nil {
		t.Error("the full package must reject an out-of-range rating")
	}
	// An editorial takes no rating, so the stored default cannot veto it.
	if _, err := bad.Normalize(string(model.ArtifactEditorial)); err != nil {
		t.Errorf("an editorial must not be rejected for a rating it does not use: %v", err)
	}
}

// Same for the test-case count, which only the scopes that produce tests use.
func TestNormalizeValidatesTheTestCaseCountOnlyWhereItIsUsed(t *testing.T) {
	bad := Request{Tags: []string{"dp"}, Rating: 1400, TestCaseCount: 0}

	if _, err := bad.Normalize(string(model.ArtifactTests)); err == nil {
		t.Error("the tests scope must reject a test-case count below 1")
	}
	if _, err := bad.Normalize(string(model.ArtifactEditorial)); err != nil {
		t.Errorf("an editorial must not be rejected for a test-case count it does not use: %v", err)
	}
}

// An unknown artifact is rejected here, which is what lets JobService.Create drop
// its own separate whitelist: one list of the artifact types, not two.
func TestNormalizeRejectsAnUnknownArtifact(t *testing.T) {
	_, err := Request{Tags: []string{"dp"}, Rating: 1400, TestCaseCount: 10}.Normalize("nonsense")
	if err == nil {
		t.Fatal("an unknown artifact type must be rejected")
	}
	if !strings.Contains(err.Error(), "unsupported artifact_type") {
		t.Errorf("the error should name the problem: %v", err)
	}
}

// The rules that are not about a specific scope still apply wherever the fields
// they constrain are in play.
func TestNormalizeKeepsTheProfileRules(t *testing.T) {
	// ICPC + subtasks is a contradiction, but only a scope that can ask for
	// subtasks carries it.
	icpcSubtasks := Request{Tags: []string{"dp"}, Rating: 1400, TestCaseCount: 10, FormatProfile: ProfileICPC, HasSubtasks: true}
	if _, err := icpcSubtasks.Normalize(string(model.ArtifactTests)); err == nil {
		t.Error("the tests scope must reject ICPC with subtasks")
	}

	// ICPC forces team/onsite, as it always did.
	icpc := Request{Tags: []string{"dp"}, Rating: 1400, TestCaseCount: 10, FormatProfile: ProfileICPC}
	got, err := icpc.Normalize(string(model.ArtifactFullProblem))
	if err != nil {
		t.Fatalf("ICPC profile rejected: %v", err)
	}
	if got.SolverMode != SolverTeam || got.TeamSize != 3 || got.EventContext != EventOnsite {
		t.Errorf("ICPC must force a team of 3 onsite, got %s/%d/%s", got.SolverMode, got.TeamSize, got.EventContext)
	}

	if _, err := (Request{Tags: []string{"dp"}, Rating: 1400, TestCaseCount: 10, FormatProfile: "nonsense"}).Normalize(string(model.ArtifactFullProblem)); err == nil {
		t.Error("an unsupported format profile must be rejected")
	}
}

// A valid brief must survive normalization for every artifact, and the defaults
// the form relies on must be filled.
func TestNormalizeAcceptsAValidBriefForEveryArtifact(t *testing.T) {
	for _, a := range model.ArtifactIDs() {
		got, err := (Request{Tags: []string{"dp"}, Rating: 1400, TestCaseCount: 10}).Normalize(a)
		if err != nil {
			t.Errorf("%s rejected a valid brief: %v", a, err)
			continue
		}
		if got.ProblemType != "standard" {
			t.Errorf("%s: problem_type = %q, want standard", a, got.ProblemType)
		}
		// The profile default is filled for every scope: it is a stored value
		// rather than a scoped input, so nothing branches on it here.
		if got.FormatProfile == "" {
			t.Errorf("%s: format_profile was not defaulted", a)
		}
	}
}

// The rating range is defaulted only where the rating is. The full package is
// the one scope that takes it.
func TestNormalizeDefaultsTheRatingRangeForTheFullPackage(t *testing.T) {
	got, err := (Request{Tags: []string{"dp"}, Rating: 1400, TestCaseCount: 10}).Normalize(string(model.ArtifactFullProblem))
	if err != nil {
		t.Fatalf("full package rejected: %v", err)
	}
	if got.RatingMin != 1300 || got.RatingMax != 1500 {
		t.Errorf("range = %d-%d, want 1300-1500", got.RatingMin, got.RatingMax)
	}
}

// The rating range is only defaulted where the rating is: an artifact that takes
// its difficulty from the existing problem must not grow a range it never asked
// for, or the stored brief would describe a calibration that does not apply.
func TestNormalizeDoesNotDefaultARangeForAnArtifactWithoutARating(t *testing.T) {
	got, err := (Request{Rating: 1400, TestCaseCount: 10}).Normalize(string(model.ArtifactEditorial))
	if err != nil {
		t.Fatalf("editorial rejected: %v", err)
	}
	if got.RatingMin != 0 || got.RatingMax != 0 {
		t.Errorf("an editorial must not be given a rating range, got %d-%d", got.RatingMin, got.RatingMax)
	}
}

// Submission through the service must reject an unknown artifact too — the
// whitelist moved into Normalize rather than disappearing.
func TestCreateRejectsAnUnknownArtifact(t *testing.T) {
	s := &JobService{}
	_, err := s.Create(t.Context(), "actor", CreateJobRequest{
		ArtifactType: "not-a-thing",
		Brief:        Request{Tags: []string{"dp"}, Rating: 1400, TestCaseCount: 10},
	})
	if err == nil {
		t.Fatal("Create accepted an unknown artifact type")
	}
}
