package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// TestStaffedByContest is the store half of "a judge may open a problem attached
// to the round they run".
//
// The bug: contest_problems deliberately skips p.visible so a private problem
// still opens during a round, but the public problem page answered 403 for that
// same draft to a judge who did not author it. One setter attaching a problem left
// the rest of the round's staff unable to read it.
//
// The predicate is narrow on purpose, and the negative cases below are the ones
// that matter: a staff row on a *different* contest must not open this problem, and
// being the problem's author is not what this answers.
func TestStaffedByContest(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	users := NewUserStore(db)
	contests := NewContestStore(db)
	problems := NewProblemStore(db)

	newUser := func(role string) *model.User {
		u := &model.User{
			ID:           uuid.New().String(),
			Username:     "sc_" + uuid.New().String()[:8],
			Email:        "sc_" + uuid.New().String()[:8] + "@example.com",
			PasswordHash: "hashed",
			Role:         role,
		}
		if err := users.Create(ctx, u); err != nil {
			t.Fatalf("create user: %v", err)
		}
		return u
	}

	author := newUser(model.RoleSetter)
	// The round's creator is a separate person from the problem's author, because
	// Create grants the creator a manager row — which makes them staff for a reason
	// unrelated to authorship. Keeping them distinct is what lets this test assert
	// that authorship alone does not answer the staff question.
	organizer := newUser(model.RoleSetter)
	judge := newUser(model.RoleUser)
	outsider := newUser(model.RoleUser)
	manager := newUser(model.RoleUser)
	tester := newUser(model.RoleUser)
	for _, u := range []*model.User{author, organizer, judge, outsider, manager, tester} {
		defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)
	}

	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	newContest := func(title string) *model.Contest {
		c := &model.Contest{
			ID: uuid.New().String(), Title: title, Type: "acm", Format: "acm",
			StartTime: start, EndTime: start.Add(time.Hour), CreatedBy: organizer.ID,
			Status: model.ContestStatusDraft, IsRated: true, RatedForMax: 9999,
		}
		if err := contests.Create(ctx, c); err != nil {
			t.Fatalf("create contest: %v", err)
		}
		// Cleanup is the caller's business: a defer here would fire when this
		// closure returns, deleting the round before it is ever used.
		return c
	}

	round := newContest("Round")
	otherRound := newContest("Other round")
	for _, c := range []*model.Contest{round, otherRound} {
		defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", c.ID)
	}

	draft := &model.Problem{
		ID: uuid.New().String(), Slug: "sc_" + uuid.New().String()[:8], Title: "Draft",
		Description: "d", InputFormat: "i", OutputFormat: "o",
		TimeLimit: 1000, MemoryLimit: 262144, Visible: false, CreatedBy: author.ID,
	}
	if err := problems.Create(ctx, draft); err != nil {
		t.Fatalf("create problem: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM problems WHERE id = $1", draft.ID)

	if err := contests.AddProblem(ctx, round.ID, draft.ID, "A", 100, 0); err != nil {
		t.Fatalf("attach problem: %v", err)
	}
	for _, tc := range []struct {
		user  *model.User
		level string
	}{
		{judge, model.ContestPermissionJudge},
		{manager, model.ContestPermissionManager},
		{tester, model.ContestPermissionTester},
	} {
		if err := contests.AddPermission(ctx, round.ID, tc.user.ID, tc.level); err != nil {
			t.Fatalf("grant %s: %v", tc.level, err)
		}
	}
	// A staff row on a different contest must grant nothing here.
	if err := contests.AddPermission(ctx, otherRound.ID, outsider.ID, model.ContestPermissionJudge); err != nil {
		t.Fatalf("grant outsider: %v", err)
	}

	cases := []struct {
		name string
		user *model.User
		want bool
	}{
		{"judge of the round may read it", judge, true},
		{"manager of the round may read it", manager, true},
		{"tester of the round may read it", tester, true},
		// Being the author is answered by problems.created_by, not by this query.
		{"the author is not staff", author, false},
		{"a judge of a different round may not", outsider, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := problems.StaffedByContest(ctx, draft.ID, tc.user.ID)
			if err != nil {
				t.Fatalf("StaffedByContest: %v", err)
			}
			if got != tc.want {
				t.Fatalf("StaffedByContest = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("an empty user id is not staff", func(t *testing.T) {
		got, err := problems.StaffedByContest(ctx, draft.ID, "")
		if err != nil {
			t.Fatalf("StaffedByContest: %v", err)
		}
		if got {
			t.Fatal("an anonymous caller must not staff anything")
		}
	})
}
