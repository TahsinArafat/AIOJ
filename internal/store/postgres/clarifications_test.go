package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/tahsinarafat/aioj/internal/model"
)

// A round's clarification queue is the jury's work list. Two things it has to get
// right:
//
//   - visibility: a private question and its reply belong to whoever asked. The
//     jury sees everything, which is the point of a nil viewer.
//   - the problem label: a question carries problem_id, but a contestant reads a
//     round by index (A, B, C…), which only exists on contest_problems. The join
//     is what lets the UI print "A — Sum problem".
func TestClarificationQueue(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	users := NewUserStore(db)
	problems := NewProblemStore(db)
	contests := NewContestStore(db)
	store := NewClarificationStore(db)

	owner := &model.User{
		ID: uuid.New().String(), Username: "clr_" + uuid.New().String()[:8],
		Email: uuid.New().String()[:8] + "@ex.com", PasswordHash: "hashed", Role: "setter",
	}
	if err := users.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)

	contest := &model.Contest{
		ID: uuid.New().String(), Title: "Clarify Cup", Type: "acm", Format: "acm",
		CreatedBy: owner.ID, Status: model.ContestStatusDraft,
	}
	if err := contests.Create(ctx, contest); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", contest.ID)

	problem := &model.Problem{
		ID: uuid.New().String(), Slug: "clr-" + uuid.New().String()[:8], Title: "Sum",
		Description: "add", TimeLimit: 1000, MemoryLimit: 262144, Difficulty: "easy",
		Tags: []string{}, CreatedBy: owner.ID, Visible: true,
	}
	if err := problems.Create(ctx, problem); err != nil {
		t.Fatalf("create problem: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM problems WHERE id = $1", problem.ID)

	if err := contests.AddProblem(ctx, contest.ID, problem.ID, "A", 100, 0); err != nil {
		t.Fatalf("add problem to contest: %v", err)
	}

	asker := &model.User{
		ID: uuid.New().String(), Username: "ask_" + uuid.New().String()[:8],
		Email: uuid.New().String()[:8] + "@ex.com", PasswordHash: "hashed", Role: "user",
	}
	if err := users.Create(ctx, asker); err != nil {
		t.Fatalf("create asker: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", asker.ID)

	otherUser := &model.User{
		ID: uuid.New().String(), Username: "oth_" + uuid.New().String()[:8],
		Email: uuid.New().String()[:8] + "@ex.com", PasswordHash: "hashed", Role: "user",
	}
	if err := users.Create(ctx, otherUser); err != nil {
		t.Fatalf("create other user: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", otherUser.ID)

	pid := problem.ID
	publicQuestion := &model.Clarification{
		ContestID: contest.ID, UserID: asker.ID, ProblemID: &pid, Question: "Is n inclusive?",
	}
	privateQuestion := &model.Clarification{
		ContestID: contest.ID, UserID: otherUser.ID, ProblemID: &pid, Question: "Can I submit twice?",
	}
	for _, c := range []*model.Clarification{publicQuestion, privateQuestion} {
		if err := store.Create(ctx, c); err != nil {
			t.Fatalf("create clarification: %v", err)
		}
		defer db.ExecContext(ctx, "DELETE FROM clarifications WHERE id = $1", c.ID)
	}

	t.Run("labels a question with the contest problem index", func(t *testing.T) {
		items, err := store.ListByContest(ctx, contest.ID, nil, nil, nil)
		if err != nil {
			t.Fatalf("ListByContest: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("jury should see every question, got %d", len(items))
		}
		for _, c := range items {
			if c.ProblemIdx == nil || *c.ProblemIdx != "A" {
				t.Fatalf("problem index for %q = %v, want A", c.Question, c.ProblemIdx)
			}
		}
	})

	t.Run("a private question is hidden from other contestants", func(t *testing.T) {
		// asker's own: both theirs and any public one.
		own, err := store.ListByContest(ctx, contest.ID, &asker.ID, nil, nil)
		if err != nil {
			t.Fatalf("ListByContest(asker): %v", err)
		}
		for _, c := range own {
			if c.UserID == otherUser.ID {
				t.Fatalf("asker saw another contestant's private question: %+v", c)
			}
		}
	})

	t.Run("filters to unanswered", func(t *testing.T) {
		if err := store.Answer(ctx, publicQuestion.ID, "Yes, inclusive.", owner.ID, true); err != nil {
			t.Fatalf("answer: %v", err)
		}
		defer db.ExecContext(ctx, "UPDATE clarifications SET answer = NULL WHERE id = $1", publicQuestion.ID)

		open := false
		items, err := store.ListByContest(ctx, contest.ID, nil, nil, &open)
		if err != nil {
			t.Fatalf("ListByContest(open): %v", err)
		}
		if len(items) != 1 || items[0].ID != privateQuestion.ID {
			t.Fatalf("unanswered should be just the private one, got %+v", items)
		}
	})

	t.Run("answers many with one ruling", func(t *testing.T) {
		ids := []string{publicQuestion.ID, privateQuestion.ID}
		if err := store.AnswerMany(ctx, ids, "Please see the FAQ.", owner.ID, false); err != nil {
			t.Fatalf("AnswerMany: %v", err)
		}
		defer db.ExecContext(ctx, "UPDATE clarifications SET answer = NULL, is_public = false WHERE id = ANY($1)",
			pq.Array(ids))

		items, err := store.ListByContest(ctx, contest.ID, nil, nil, nil)
		if err != nil {
			t.Fatalf("ListByContest: %v", err)
		}
		for _, c := range items {
			if c.Answer == nil || *c.Answer != "Please see the FAQ." {
				t.Fatalf("question %q was not answered: %+v", c.Question, c.Answer)
			}
		}
	})
}
