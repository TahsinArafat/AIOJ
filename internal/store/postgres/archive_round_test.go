package postgres

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * A problem may sit in several contests at once, and only an *open* one takes it
 * out of the archive. This is what the predicate's two extra conditions buy,
 * checked end to end against the real queries.
 *
 * Both bugs it pins were silent catalogue losses: an upcoming round hid its
 * problems the day they were attached, and an unpublished draft round hid a
 * published problem forever with nothing on screen to explain the gap.
 */
type archiveRoundFixture struct {
	db          *sql.DB
	ownerID     string
	problemID   string
	problemSlug string
	// contestIDs by label, for the assertions.
	contestIDs map[string]string
}

func newArchiveRoundFixture(t *testing.T, ctx context.Context) (*archiveRoundFixture, func()) {
	t.Helper()
	db := openTestDB(t)

	users := NewUserStore(db)
	problems := NewProblemStore(db)
	contests := NewContestStore(db)

	f := &archiveRoundFixture{db: db, contestIDs: map[string]string{}}

	f.ownerID = uuid.NewString()
	if err := users.Create(ctx, &model.User{
		ID: f.ownerID, Username: "setter_" + uuid.NewString()[:8],
		Email: uuid.NewString() + "@aioj.test", PasswordHash: "x", Role: "setter",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	f.problemID = uuid.NewString()
	f.problemSlug = "archived-" + uuid.NewString()[:8]
	if err := problems.Create(ctx, &model.Problem{
		ID: f.problemID, Slug: f.problemSlug, Title: "Archived Problem",
		Description: "d", TimeLimit: 1000, MemoryLimit: 262144, Difficulty: "easy",
		Tags: []string{}, CreatedBy: f.ownerID,
		Status: model.ProblemStatusPublished, Visible: true,
	}); err != nil {
		t.Fatalf("create problem: %v", err)
	}

	// rounds pairs a label with the contest it describes.
	now := time.Now().UTC()
	rounds := []struct {
		label     string
		visible   bool
		startTime time.Time
		endTime   time.Time
	}{
		{"running", true, now.Add(-time.Hour), now.Add(time.Hour)},
		{"finished", true, now.Add(-3 * time.Hour), now.Add(-time.Hour)},
		{"upcoming", true, now.Add(24 * time.Hour), now.Add(26 * time.Hour)},
		// An unpublished round: SetReviewState keeps visible false for anything that
		// is not 'published', so a draft conceals nothing.
		{"draft", false, now.Add(-time.Hour), now.Add(time.Hour)},
	}
	for _, round := range rounds {
		id := uuid.NewString()
		c := &model.Contest{
			ID: id, Title: round.label + " " + uuid.NewString()[:6],
			Type: "acm", Format: "acm",
			StartTime: round.startTime, EndTime: round.endTime,
			CreatedBy: f.ownerID, Visible: round.visible,
		}
		if round.visible {
			c.Status = model.ContestStatusPublished
		}
		if err := contests.Create(ctx, c); err != nil {
			t.Fatalf("create %s contest: %v", round.label, err)
		}
		if err := contests.AddProblem(ctx, id, f.problemID, "A", 100, 0); err != nil {
			t.Fatalf("attach problem to %s contest: %v", round.label, err)
		}
		f.contestIDs[round.label] = id
	}

	cleanup := func() {
		db.ExecContext(ctx, "DELETE FROM contest_problems WHERE problem_id = $1", f.problemID)
		db.ExecContext(ctx, "DELETE FROM contests WHERE id = ANY($1)", f.allContestIDs())
		db.ExecContext(ctx, "DELETE FROM problems WHERE id = $1", f.problemID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", f.ownerID)
		db.Close()
	}
	return f, cleanup
}

func (f *archiveRoundFixture) allContestIDs() []string {
	ids := make([]string, 0, len(f.contestIDs))
	for _, id := range f.contestIDs {
		ids = append(ids, id)
	}
	return ids
}

// detachEvery removes the problem from every round. Each subtest then re-attaches
// just the rounds it is about, so the cases cannot inherit each other's state.
func (f *archiveRoundFixture) detachEvery(t *testing.T, ctx context.Context) {
	t.Helper()
	if _, err := f.db.ExecContext(ctx,
		"DELETE FROM contest_problems WHERE problem_id = $1", f.problemID); err != nil {
		t.Fatalf("detach problem: %v", err)
	}
}

func (f *archiveRoundFixture) attach(t *testing.T, ctx context.Context, label string) {
	t.Helper()
	if _, err := f.db.ExecContext(ctx,
		"INSERT INTO contest_problems(contest_id, problem_id, index, score, sort_order) VALUES($1,$2,'A',100,0)",
		f.contestIDs[label], f.problemID); err != nil {
		t.Fatalf("attach to %s contest: %v", label, err)
	}
}

func TestUnreleasedContestOnlyWhileTheRoundIsOpen(t *testing.T) {
	ctx := context.Background()
	f, cleanup := newArchiveRoundFixture(t, ctx)
	defer cleanup()

	problems := NewProblemStore(f.db)

	for _, tc := range []struct {
		label string
		keep  []string
		want  bool
		why   string
	}{
		{"running round", []string{"running"}, true, "a round under way must keep its problem out of the archive"},
		{"finished round", []string{"finished"}, false, "an ended round gives the problem back"},
		{"upcoming round", []string{"upcoming"}, false, "a round that has not started spoils nothing yet"},
		{"draft round", []string{"draft"}, false, "an unpublished round nobody can enter conceals nothing"},
		{"no contest at all", nil, false, "an ordinary archive problem stays in the archive"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			f.detachEvery(t, ctx)
			for _, label := range tc.keep {
				f.attach(t, ctx, label)
			}

			got, err := problems.InUnreleasedContest(ctx, f.problemID)
			if err != nil {
				t.Fatalf("InUnreleasedContest: %v", err)
			}
			if got != tc.want {
				t.Fatalf("InUnreleasedContest = %v, want %v: %s", got, tc.want, tc.why)
			}
		})
	}
}

// The archive list is the surface the loss was noticed on, so the shared
// predicate is asserted through it rather than through the helper alone.
func TestArchivedProblemListAgreesWithThePredicate(t *testing.T) {
	ctx := context.Background()
	f, cleanup := newArchiveRoundFixture(t, ctx)
	defer cleanup()

	problems := NewProblemStore(f.db)

	listed := func() bool {
		t.Helper()
		items, _, err := problems.List(ctx, 0, 500, false)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, item := range items {
			if item.Slug == f.problemSlug {
				return true
			}
		}
		return false
	}

	f.detachEvery(t, ctx)
	f.attach(t, ctx, "upcoming")
	if !listed() {
		t.Error("a published problem in an upcoming round must still be listed")
	}

	f.detachEvery(t, ctx)
	f.attach(t, ctx, "running")
	if listed() {
		t.Error("a problem in a round under way must not be listed")
	}

	f.detachEvery(t, ctx)
	if !listed() {
		t.Error("an ordinary archive problem must be listed")
	}
}
