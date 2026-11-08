package postgres

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// statusFixture builds one owner user, one visible problem, one hidden problem
// and a finished plus a running contest, with a submission in each situation.
type statusFixture struct {
	db               *sql.DB
	ownerID, otherID string
	visibleID        string
	hiddenID         string
	doneContestID    string
	liveContestID    string
	// visibleSlug names the visible problem so the tests can filter by it.
	visibleSlug string
	subs        map[string]string // label -> submission id
}

func newStatusFixture(t *testing.T, ctx context.Context) (*statusFixture, func()) {
	t.Helper()
	db := openTestDB(t)

	users := NewUserStore(db)
	problems := NewProblemStore(db)
	contests := NewContestStore(db)
	subs := NewSubmissionStore(db)

	f := &statusFixture{db: db, subs: map[string]string{}}

	f.ownerID = uuid.NewString()
	f.otherID = uuid.NewString()
	for _, u := range []*model.User{
		{ID: f.ownerID, Username: "owner_" + uuid.NewString()[:8], Email: uuid.NewString() + "@aioj.test", PasswordHash: "x", Role: "setter"},
		{ID: f.otherID, Username: "solver_" + uuid.NewString()[:8], Email: uuid.NewString() + "@aioj.test", PasswordHash: "x", Role: "user"},
	} {
		if err := users.Create(ctx, u); err != nil {
			t.Fatalf("create user: %v", err)
		}
	}

	f.visibleSlug = "visible-" + uuid.NewString()[:8]
	f.visibleID = uuid.NewString()
	if err := problems.Create(ctx, &model.Problem{
		ID: f.visibleID, Slug: f.visibleSlug, Title: "Visible Problem",
		Description: "d", TimeLimit: 1000, MemoryLimit: 262144, Difficulty: "easy",
		Tags: []string{}, CreatedBy: f.ownerID, Visible: true,
	}); err != nil {
		t.Fatalf("create visible problem: %v", err)
	}

	f.hiddenID = uuid.NewString()
	if err := problems.Create(ctx, &model.Problem{
		ID: f.hiddenID, Slug: "hidden-" + uuid.NewString()[:8], Title: "Hidden Problem",
		Description: "d", TimeLimit: 1000, MemoryLimit: 262144, Difficulty: "easy",
		Tags: []string{}, CreatedBy: f.ownerID, Visible: false,
	}); err != nil {
		t.Fatalf("create hidden problem: %v", err)
	}

	now := time.Now().UTC()
	f.doneContestID = uuid.NewString()
	if err := contests.Create(ctx, &model.Contest{
		ID: f.doneContestID, Title: "Finished " + uuid.NewString()[:6], Type: "acm", Format: "acm",
		StartTime: now.Add(-3 * time.Hour), EndTime: now.Add(-time.Hour), CreatedBy: f.ownerID,
	}); err != nil {
		t.Fatalf("create finished contest: %v", err)
	}
	f.liveContestID = uuid.NewString()
	if err := contests.Create(ctx, &model.Contest{
		ID: f.liveContestID, Title: "Running " + uuid.NewString()[:6], Type: "acm", Format: "acm",
		StartTime: now.Add(-10 * time.Minute), EndTime: now.Add(time.Hour), CreatedBy: f.ownerID,
	}); err != nil {
		t.Fatalf("create running contest: %v", err)
	}

	add := func(label, problemID, userID, contestID string, status model.SubmissionStatus) {
		id := uuid.NewString()
		if err := subs.Create(ctx, &model.Submission{
			ID: id, ProblemID: problemID, UserID: userID, ContestID: contestID,
			Language: "cpp-gpp-64", SourceCode: "int main(){}", Status: status,
			SubmissionType: model.SubmissionTypeCode,
		}); err != nil {
			t.Fatalf("create submission %s: %v", label, err)
		}
		f.subs[label] = id
	}

	add("practice_ac", f.visibleID, f.otherID, "", model.StatusAC)
	add("practice_wa", f.visibleID, f.ownerID, "", model.StatusWA)
	add("hidden", f.hiddenID, f.otherID, "", model.StatusAC)
	add("finished_contest", f.visibleID, f.otherID, f.doneContestID, model.StatusAC)
	add("live_contest", f.visibleID, f.otherID, f.liveContestID, model.StatusTLE)
	add("pending", f.visibleID, f.otherID, "", model.StatusPending)

	cleanup := func() {
		db.ExecContext(ctx, "DELETE FROM submissions WHERE problem_id IN ($1,$2)", f.visibleID, f.hiddenID)
		db.ExecContext(ctx, "DELETE FROM contests WHERE id IN ($1,$2)", f.doneContestID, f.liveContestID)
		db.ExecContext(ctx, "DELETE FROM problems WHERE id IN ($1,$2)", f.visibleID, f.hiddenID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id IN ($1,$2)", f.ownerID, f.otherID)
		db.Close()
	}
	return f, cleanup
}

func idsOf(rows []model.Submission) map[string]bool {
	out := make(map[string]bool, len(rows))
	for _, r := range rows {
		out[r.ID] = true
	}
	return out
}

// The status page must not leak what the reader cannot open: rows on hidden
// problems, and rows from a contest that is still running.
func TestListPublicVisibility(t *testing.T) {
	ctx := context.Background()
	f, cleanup := newStatusFixture(t, ctx)
	defer cleanup()

	store := NewSubmissionStore(f.db)

	// Filter to this fixture's problems so other rows in the shared test DB do
	// not affect the assertions.
	list := func(filter model.PublicSubmissionFilter) map[string]bool {
		t.Helper()
		rows, _, err := store.ListPublic(ctx, 0, 100, filter)
		if err != nil {
			t.Fatalf("ListPublic: %v", err)
		}
		return idsOf(rows)
	}

	t.Run("anonymous reader sees practice rows on visible problems only", func(t *testing.T) {
		got := list(model.PublicSubmissionFilter{SubmissionFilter: model.SubmissionFilter{ProblemID: f.visibleID}})
		if !got[f.subs["practice_ac"]] || !got[f.subs["practice_wa"]] {
			t.Errorf("expected both practice rows, got %v", got)
		}
		if got[f.subs["live_contest"]] {
			t.Error("row from a running contest must not be public")
		}
		if !got[f.subs["finished_contest"]] {
			t.Error("row from a finished contest must be public")
		}
	})

	t.Run("hidden problem rows are not public", func(t *testing.T) {
		got := list(model.PublicSubmissionFilter{SubmissionFilter: model.SubmissionFilter{ProblemID: f.hiddenID}})
		if got[f.subs["hidden"]] {
			t.Error("row on a hidden problem must not be public")
		}
	})

	t.Run("viewer still sees their own rows on a hidden problem", func(t *testing.T) {
		got := list(model.PublicSubmissionFilter{
			SubmissionFilter: model.SubmissionFilter{ProblemID: f.hiddenID},
			ViewerID:         f.otherID,
		})
		if !got[f.subs["hidden"]] {
			t.Error("a user must always see their own submission")
		}
	})

	t.Run("viewer still sees their own rows inside a running contest", func(t *testing.T) {
		got := list(model.PublicSubmissionFilter{
			SubmissionFilter: model.SubmissionFilter{ProblemID: f.visibleID},
			ViewerID:         f.otherID,
		})
		if !got[f.subs["live_contest"]] {
			t.Error("a contestant must see their own row during the contest")
		}
	})

	t.Run("admin sees everything", func(t *testing.T) {
		got := list(model.PublicSubmissionFilter{
			SubmissionFilter: model.SubmissionFilter{ProblemID: f.hiddenID},
			ViewerID:         f.ownerID,
			ShowAll:          true,
		})
		if !got[f.subs["hidden"]] {
			t.Error("admin must see rows on hidden problems")
		}
	})

	t.Run("filters by slug, handle and verdict", func(t *testing.T) {
		bySlug := list(model.PublicSubmissionFilter{ProblemSlug: f.visibleSlug})
		if !bySlug[f.subs["practice_ac"]] || bySlug[f.subs["hidden"]] {
			t.Errorf("problem slug filter returned the wrong rows: %v", bySlug)
		}

		rows, total, err := store.ListPublic(ctx, 0, 100, model.PublicSubmissionFilter{
			SubmissionFilter: model.SubmissionFilter{ProblemID: f.visibleID, Status: string(model.StatusWA)},
		})
		if err != nil {
			t.Fatalf("ListPublic(status): %v", err)
		}
		if total != 1 || len(rows) != 1 || rows[0].ID != f.subs["practice_wa"] {
			t.Fatalf("verdict filter: want the single WA row, got total=%d rows=%d", total, len(rows))
		}
		if rows[0].ProblemSlug != f.visibleSlug || rows[0].ProblemTitle != "Visible Problem" {
			t.Errorf("listing must carry the problem name, got slug=%q title=%q", rows[0].ProblemSlug, rows[0].ProblemTitle)
		}
		if rows[0].Username == "" {
			t.Error("listing must carry the submitter's username")
		}
	})

	t.Run("pagination does not repeat rows", func(t *testing.T) {
		filter := model.PublicSubmissionFilter{SubmissionFilter: model.SubmissionFilter{ProblemID: f.visibleID}}
		first, _, err := store.ListPublic(ctx, 0, 2, filter)
		if err != nil {
			t.Fatalf("ListPublic page 1: %v", err)
		}
		second, _, err := store.ListPublic(ctx, 2, 2, filter)
		if err != nil {
			t.Fatalf("ListPublic page 2: %v", err)
		}
		seen := idsOf(first)
		for _, row := range second {
			if seen[row.ID] {
				t.Errorf("row %s appeared on both pages", row.ID)
			}
		}
	})
}

// The setter listing is the surface the rejudge buttons act on, so it has to
// support the same filters and name the submitter.
func TestListByProblemManage(t *testing.T) {
	ctx := context.Background()
	f, cleanup := newStatusFixture(t, ctx)
	defer cleanup()
	store := NewSubmissionStore(f.db)

	all, total, err := store.ListByProblemManage(ctx, f.visibleID, 0, 100, model.SubmissionFilter{})
	if err != nil {
		t.Fatalf("ListByProblemManage: %v", err)
	}
	// practice_ac, practice_wa, finished_contest, live_contest, pending
	if total != 5 || len(all) != 5 {
		t.Fatalf("want 5 rows for the visible problem, got total=%d rows=%d", total, len(all))
	}
	for _, row := range all {
		if row.Username == "" {
			t.Errorf("row %s has no username", row.ID)
		}
		if row.ProblemSlug != f.visibleSlug {
			t.Errorf("row %s has slug %q", row.ID, row.ProblemSlug)
		}
	}

	rows, total, err := store.ListByProblemManage(ctx, f.visibleID, 0, 100, model.SubmissionFilter{ContestID: f.liveContestID})
	if err != nil {
		t.Fatalf("ListByProblemManage(contest): %v", err)
	}
	if total != 1 || rows[0].ID != f.subs["live_contest"] {
		t.Fatalf("contest filter: want the live contest row, got total=%d", total)
	}

	rows, _, err = store.ListByProblemManage(ctx, f.visibleID, 0, 100, model.SubmissionFilter{Status: string(model.StatusAC)})
	if err != nil {
		t.Fatalf("ListByProblemManage(status): %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("verdict filter: want 2 AC rows, got %d", len(rows))
	}
}

// A batch rejudge must skip rows that are already queued and remote rows.
func TestListIDsForProblemRejudge(t *testing.T) {
	ctx := context.Background()
	f, cleanup := newStatusFixture(t, ctx)
	defer cleanup()
	store := NewSubmissionStore(f.db)

	ids, err := store.ListIDsForProblemRejudge(ctx, f.visibleID, model.SubmissionFilter{})
	if err != nil {
		t.Fatalf("ListIDsForProblemRejudge: %v", err)
	}
	got := map[string]bool{}
	for _, id := range ids {
		got[id] = true
	}
	if got[f.subs["pending"]] {
		t.Error("a pending row must not be enqueued twice")
	}
	if !got[f.subs["practice_ac"]] || !got[f.subs["live_contest"]] {
		t.Error("judged local rows must be included")
	}

	scoped, err := store.ListIDsForProblemRejudge(ctx, f.visibleID, model.SubmissionFilter{ContestID: f.doneContestID})
	if err != nil {
		t.Fatalf("ListIDsForProblemRejudge(contest): %v", err)
	}
	if len(scoped) != 1 || scoped[0] != f.subs["finished_contest"] {
		t.Fatalf("contest scope: want only the finished-contest row, got %v", scoped)
	}
}

// Rejudging does not touch the per-problem counters, so they are rebuilt from
// the submissions table instead of being incremented again.
func TestRecomputeProblemCounts(t *testing.T) {
	ctx := context.Background()
	f, cleanup := newStatusFixture(t, ctx)
	defer cleanup()
	store := NewSubmissionStore(f.db)
	problems := NewProblemStore(f.db)

	if err := problems.UpdateCounts(ctx, f.visibleID, 1000, 500); err != nil {
		t.Fatalf("UpdateCounts: %v", err)
	}
	if err := store.RecomputeProblemCounts(ctx, f.visibleID); err != nil {
		t.Fatalf("RecomputeProblemCounts: %v", err)
	}

	problem, err := problems.GetByID(ctx, f.visibleID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	// 5 submissions on the visible problem, of which practice_ac and
	// finished_contest are accepted.
	if problem.SubmissionCount != 5 || problem.AcceptedCount != 2 {
		t.Fatalf("want 5 submissions / 2 accepted, got %d / %d", problem.SubmissionCount, problem.AcceptedCount)
	}
}

func TestListAccessLevels(t *testing.T) {
	ctx := context.Background()
	f, cleanup := newStatusFixture(t, ctx)
	defer cleanup()
	problems := NewProblemStore(f.db)

	// problems.Create writes the owner permission row.
	levels, err := problems.ListAccessLevels(ctx, f.ownerID, []string{f.visibleID, f.hiddenID})
	if err != nil {
		t.Fatalf("ListAccessLevels: %v", err)
	}
	if levels[f.visibleID] != "owner" || levels[f.hiddenID] != "owner" {
		t.Fatalf("owner levels: %v", levels)
	}

	other, err := problems.ListAccessLevels(ctx, f.otherID, []string{f.visibleID})
	if err != nil {
		t.Fatalf("ListAccessLevels(other): %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("a solver has no problem permission rows, got %v", other)
	}
}

// A System Error must keep the judge's own failure text in judge_error, not in
// compile_output where the UI would read it as a compile failure — and a
// rejudge has to clear it again.
func TestSystemErrorKeepsJudgeErrorText(t *testing.T) {
	ctx := context.Background()
	f, cleanup := newStatusFixture(t, ctx)
	defer cleanup()
	store := NewSubmissionStore(f.db)

	const failure = "execution client error: dial tcp judge:5050: connect: connection refused"

	if err := store.UpdateResult(ctx, f.subs["practice_wa"], model.StatusSE, 0, 0, 0, failure, nil); err != nil {
		t.Fatalf("UpdateResult(SE): %v", err)
	}
	sub, err := store.GetByID(ctx, f.subs["practice_wa"])
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if sub.JudgeError != failure {
		t.Fatalf("judge_error = %q, want the failure text", sub.JudgeError)
	}

	// A real verdict must not carry a stale judge error.
	if err := store.UpdateResult(ctx, f.subs["practice_ac"], model.StatusAC, 100, 5, 1024, "", nil); err != nil {
		t.Fatalf("UpdateResult(AC): %v", err)
	}
	ac, err := store.GetByID(ctx, f.subs["practice_ac"])
	if err != nil {
		t.Fatalf("GetByID(AC): %v", err)
	}
	if ac.JudgeError != "" {
		t.Fatalf("an accepted submission must have no judge_error, got %q", ac.JudgeError)
	}

	failures, err := store.ListRecentFailures(ctx, 10)
	if err != nil {
		t.Fatalf("ListRecentFailures: %v", err)
	}
	found := false
	for _, row := range failures {
		if row.ID == f.subs["practice_wa"] {
			found = true
			if row.JudgeError != failure {
				t.Errorf("listed failure has judge_error %q", row.JudgeError)
			}
			if row.ProblemSlug != f.visibleSlug || row.Username == "" {
				t.Errorf("listed failure is missing problem/user context: slug=%q user=%q", row.ProblemSlug, row.Username)
			}
		}
		if row.Status != model.StatusSE {
			t.Errorf("row %s in the failures list has status %s", row.ID, row.Status)
		}
	}
	if !found {
		t.Fatal("the failed submission is missing from ListRecentFailures")
	}

	if err := store.ResetForRejudge(ctx, []string{f.subs["practice_wa"]}); err != nil {
		t.Fatalf("ResetForRejudge: %v", err)
	}
	reset, err := store.GetByID(ctx, f.subs["practice_wa"])
	if err != nil {
		t.Fatalf("GetByID after reset: %v", err)
	}
	if reset.JudgeError != "" {
		t.Errorf("rejudge must clear judge_error, got %q", reset.JudgeError)
	}
}
