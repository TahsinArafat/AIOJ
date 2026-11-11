package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// The feed query joins contest_problems and onsite_batch_users, either of which
// can silently break it. A mock cannot catch that, and the handler tests use a
// fake lister, so this runs the real SQL.
func TestListFeedRunsAgainstRealDB(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	subs := NewSubmissionStore(db)

	// Create a real contest rather than relying on seeded data: skipping because
	// the DB happens to be empty would mean the query never executes, which is the
	// opposite of what this test is for.
	uid := "00000000-0000-0000-0000-000000000000"
	contests := NewContestStore(db)
	c := &model.Contest{
		ID: uuid.New().String(), Title: "Feed Probe", Type: "acm", Format: "acm",
		StartTime: time.Now().Add(-time.Hour), EndTime: time.Now().Add(time.Hour),
		Status: model.ContestStatusPublished, Visible: true, CreatedBy: uid,
	}
	if err := contests.Create(ctx, c); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id=$1", c.ID)
	contestID := c.ID

	runs, next, err := subs.ListFeed(ctx, contestID, "", 10)
	if err != nil {
		t.Fatalf("ListFeed: %v", err)
	}
	t.Logf("contest %s: %d runs, next cursor %q", contestID, len(runs), next)

	// A cursor the server just returned must be accepted on the next poll.
	if next != "" {
		if _, _, err := subs.ListFeed(ctx, contestID, next, 10); err != nil {
			t.Fatalf("ListFeed with a cursor it just produced: %v", err)
		}
	}

	// A malformed cursor is a client error, not a crash.
	if _, _, err := subs.ListFeed(ctx, contestID, "garbage", 10); err == nil {
		t.Fatal("ListFeed accepted a malformed cursor")
	}
}
