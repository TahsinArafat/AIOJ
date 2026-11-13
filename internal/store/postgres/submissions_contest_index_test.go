package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// The round's letter is how a problem is identified inside a contest, and the
// contest Submissions tab is read at the desk, where a uuid is useless. The
// letter lives in contest_problems, not on the submission, so it has to be
// joined in — and because a submission can outlive its listing in the round,
// the join must not be allowed to drop the row.
func TestListByContestResolvesTheProblemLetter(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	users := NewUserStore(db)
	problems := NewProblemStore(db)
	contests := NewContestStore(db)
	subs := NewSubmissionStore(db)

	userID := uuid.NewString()
	if err := users.Create(ctx, &model.User{
		ID: userID, Username: "letter_" + uuid.NewString()[:8],
		Email: uuid.NewString() + "@aioj.test", PasswordHash: "x", Role: "user",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Two problems: one the round lists, one it does not. The second covers a
	// submission whose problem left the round.
	listedID, unlistedID := uuid.NewString(), uuid.NewString()
	for _, p := range []struct {
		id   string
		slug string
		ti   string
	}{{listedID, "listed-" + uuid.NewString()[:8], "Listed"}, {unlistedID, "unlisted-" + uuid.NewString()[:8], "Unlisted"}} {
		if err := problems.Create(ctx, &model.Problem{
			ID: p.id, Slug: p.slug, Title: p.ti,
			Description: "d", TimeLimit: 1000, MemoryLimit: 262144, Difficulty: "easy",
			Tags: []string{}, CreatedBy: userID,
			Status: model.ProblemStatusPublished, Visible: true,
		}); err != nil {
			t.Fatalf("create problem: %v", err)
		}
	}

	now := time.Now().UTC()
	contestID := uuid.NewString()
	if err := contests.Create(ctx, &model.Contest{
		ID: contestID, Title: "Letter " + uuid.NewString()[:6], Type: "acm", Format: "acm",
		StartTime: now.Add(-3 * time.Hour), EndTime: now.Add(-time.Hour), CreatedBy: userID,
	}); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	// 'B', not 'A': asserting a non-default letter means a bug that hands back
	// the first letter of the round, or a constant, fails here.
	if err := contests.AddProblem(ctx, contestID, listedID, "B", 500, 1); err != nil {
		t.Fatalf("add problem to contest: %v", err)
	}

	listedSub, unlistedSub := uuid.NewString(), uuid.NewString()
	for _, s := range []struct {
		id        string
		problemID string
	}{{listedSub, listedID}, {unlistedSub, unlistedID}} {
		if err := subs.Create(ctx, &model.Submission{
			ID: s.id, ProblemID: s.problemID, UserID: userID, ContestID: contestID,
			Language: "cpp-gpp-64", SourceCode: "int main(){}", Status: model.StatusAC,
			SubmissionType: model.SubmissionTypeCode,
		}); err != nil {
			t.Fatalf("create submission: %v", err)
		}
	}

	defer func() {
		db.ExecContext(ctx, "DELETE FROM submissions WHERE contest_id=$1", contestID)
		db.ExecContext(ctx, "DELETE FROM contests WHERE id=$1", contestID)
		db.ExecContext(ctx, "DELETE FROM problems WHERE id IN ($1,$2)", listedID, unlistedID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id=$1", userID)
	}()

	rows, total, err := subs.ListByContest(ctx, contestID, 0, 50, model.SubmissionFilter{})
	if err != nil {
		t.Fatalf("ListByContest: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2 — a submission whose problem is not listed in the round must still be counted", total)
	}

	byID := make(map[string]model.Submission, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}

	// The defect this pins: every row carried no letter, so the tab fell back to
	// printing a slice of the problem's uuid.
	if got := byID[listedSub].ProblemIndex; got != "B" {
		t.Errorf("listed submission problem_index = %q, want %q — the round's letter must be resolved from contest_problems", got, "B")
	}

	// And the fallback must be an empty letter on a row that is still present,
	// not a dropped row.
	if _, ok := byID[unlistedSub]; !ok {
		t.Fatal("the submission on a problem that is no longer listed in the round disappeared; the join must be LEFT")
	}
	if got := byID[unlistedSub].ProblemIndex; got != "" {
		t.Errorf("unlisted submission problem_index = %q, want empty — there is no letter to resolve", got)
	}
}
