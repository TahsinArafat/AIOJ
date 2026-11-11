package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// TestGetProblemProgress is the regression test for a contest problem list that
// showed a bare "Solve" button: no tick for a solved problem, no indication of
// which ones had been attempted, and no sense of what the field had solved.
//
// Three viewer shapes have to stay distinct, which is the whole point of the
// separate my_* fields: anonymous (counts only), solved, and attempted-but-not-
// solved.
func TestGetProblemProgress(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userStore := NewUserStore(db)
	probStore := NewProblemStore(db)
	contestStore := NewContestStore(db)
	subStore := NewSubmissionStore(db)

	owner := &model.User{ID: uuid.New().String(), Username: "pp_" + uuid.New().String()[:8],
		Email: uuid.New().String()[:8] + "@ex.com", PasswordHash: "h", Role: "setter"}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id=$1", owner.ID)

	contest := &model.Contest{ID: uuid.New().String(), Title: "PP", Type: "acm", Format: "acm",
		CreatedBy: owner.ID, Status: model.ContestStatusDraft}
	if err := contestStore.Create(ctx, contest); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id=$1", contest.ID)

	p := &model.Problem{ID: uuid.New().String(), Slug: "pp-" + uuid.New().String()[:8], Title: "P",
		Description: "d", TimeLimit: 1000, MemoryLimit: 262144, Difficulty: "easy",
		Tags: []string{}, CreatedBy: owner.ID, Visible: true}
	if err := probStore.Create(ctx, p); err != nil {
		t.Fatalf("create problem: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM problems WHERE id=$1", p.ID)
	if err := contestStore.AddProblem(ctx, contest.ID, p.ID, "A", 100, 0); err != nil {
		t.Fatalf("add problem: %v", err)
	}

	solver := &model.User{ID: uuid.New().String(), Username: "sol_" + uuid.New().String()[:8],
		Email: uuid.New().String()[:8] + "@ex.com", PasswordHash: "h", Role: "user"}
	if err := userStore.Create(ctx, solver); err != nil {
		t.Fatalf("create solver: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id=$1", solver.ID)

	// One accepted submission from the solver, one failed attempt from the owner.
	for _, c := range []struct {
		uid    string
		status model.SubmissionStatus
	}{
		{solver.ID, model.StatusAC},
		{owner.ID, model.StatusWA},
	} {
		s := &model.Submission{ID: uuid.New().String(), ProblemID: p.ID, UserID: c.uid,
			ContestID: contest.ID, Language: "cpp", SourceCode: "x", Status: c.status,
			SubmissionType: model.SubmissionTypeCode}
		if err := subStore.Create(ctx, s); err != nil {
			t.Fatalf("create submission: %v", err)
		}
	}
	defer db.ExecContext(ctx, "DELETE FROM submissions WHERE contest_id=$1", contest.ID)

	t.Run("anonymous gets the round count but no personal state", func(t *testing.T) {
		got, err := contestStore.GetProblemProgress(ctx, contest.ID, "", nil)
		if err != nil {
			t.Fatalf("GetProblemProgress: %v", err)
		}
		if got[p.ID].SolvedCount != 1 {
			t.Fatalf("solved_count = %d, want 1", got[p.ID].SolvedCount)
		}
		if got[p.ID].MySolved || got[p.ID].MyAttempted || got[p.ID].MySubmissions != 0 {
			t.Fatalf("an anonymous viewer must carry no personal state: %+v", got[p.ID])
		}
	})

	t.Run("a solver sees the problem as solved", func(t *testing.T) {
		got, err := contestStore.GetProblemProgress(ctx, contest.ID, solver.ID, nil)
		if err != nil {
			t.Fatalf("GetProblemProgress: %v", err)
		}
		if !got[p.ID].MySolved || !got[p.ID].MyAttempted || got[p.ID].MySubmissions != 1 {
			t.Fatalf("state = %+v, want solved + attempted + 1 submission", got[p.ID])
		}
	})

	t.Run("a failed attempt is attempted but not solved", func(t *testing.T) {
		got, err := contestStore.GetProblemProgress(ctx, contest.ID, owner.ID, nil)
		if err != nil {
			t.Fatalf("GetProblemProgress: %v", err)
		}
		if got[p.ID].MySolved || !got[p.ID].MyAttempted || got[p.ID].MySubmissions != 1 {
			t.Fatalf("state = %+v, want attempted-not-solved + 1 submission", got[p.ID])
		}
	})
}

// During a standings freeze the public "solved by N" must not count solves made
// after the freeze instant, or the number gives away exactly what the frozen
// board is hiding.
func TestGetProblemProgressFreezeCutoff(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userStore := NewUserStore(db)
	probStore := NewProblemStore(db)
	contestStore := NewContestStore(db)

	owner := &model.User{ID: uuid.New().String(), Username: "fz_" + uuid.New().String()[:8],
		Email: uuid.New().String()[:8] + "@ex.com", PasswordHash: "h", Role: "setter"}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id=$1", owner.ID)

	contest := &model.Contest{ID: uuid.New().String(), Title: "FZ", Type: "acm", Format: "acm",
		CreatedBy: owner.ID, Status: model.ContestStatusDraft}
	if err := contestStore.Create(ctx, contest); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id=$1", contest.ID)

	p := &model.Problem{ID: uuid.New().String(), Slug: "fz-" + uuid.New().String()[:8], Title: "P",
		Description: "d", TimeLimit: 1000, MemoryLimit: 262144, Difficulty: "easy",
		Tags: []string{}, CreatedBy: owner.ID, Visible: true}
	if err := probStore.Create(ctx, p); err != nil {
		t.Fatalf("create problem: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM problems WHERE id=$1", p.ID)
	if err := contestStore.AddProblem(ctx, contest.ID, p.ID, "A", 100, 0); err != nil {
		t.Fatalf("add problem: %v", err)
	}

	mkUser := func(tag string) string {
		id := uuid.New().String()
		u := &model.User{ID: id, Username: tag + "_" + uuid.New().String()[:8],
			Email: uuid.New().String()[:8] + "@ex.com", PasswordHash: "h", Role: "user"}
		if err := userStore.Create(ctx, u); err != nil {
			t.Fatalf("create %s: %v", tag, err)
		}
		return id
	}
	before := mkUser("before")
	after := mkUser("after")
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id=$1", before)
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id=$1", after)

	// One solve on each side of the freeze instant.
	for _, c := range []struct {
		uid string
		at  time.Time
	}{
		{before, time.Now().Add(-2 * time.Hour)},
		{after, time.Now()},
	} {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO submissions (id,problem_id,user_id,contest_id,language,source_code,status,submission_type,created_at)
			 VALUES ($1,$2,$3,$4,'cpp','x','ac','code',$5)`,
			uuid.New().String(), p.ID, c.uid, contest.ID, c.at); err != nil {
			t.Fatalf("insert submission: %v", err)
		}
	}
	defer db.ExecContext(ctx, "DELETE FROM submissions WHERE contest_id=$1", contest.ID)

	cutoff := time.Now().Add(-time.Hour)

	live, err := contestStore.GetProblemProgress(ctx, contest.ID, "", nil)
	if err != nil {
		t.Fatalf("live: %v", err)
	}
	if live[p.ID].SolvedCount != 2 {
		t.Fatalf("live solved_count = %d, want 2", live[p.ID].SolvedCount)
	}

	frozen, err := contestStore.GetProblemProgress(ctx, contest.ID, "", &cutoff)
	if err != nil {
		t.Fatalf("frozen: %v", err)
	}
	if frozen[p.ID].SolvedCount != 1 {
		t.Fatalf("frozen solved_count = %d, want 1 (the post-freeze solve must not count)", frozen[p.ID].SolvedCount)
	}
}
