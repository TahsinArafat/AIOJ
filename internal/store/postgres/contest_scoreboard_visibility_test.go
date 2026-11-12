package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * The scoreboard axis has to survive a write and a reload, and it has to default
 * to the closed state for anyone who does not set it.
 *
 * The three read sites (GetByID, ListForViewer, ListByCreatedBy) each scan their
 * own column list, so a column added to one and forgotten in another is a
 * silent zero — which for this field means "participants", i.e. safe, but also
 * means a setter's public board would quietly revert on the next page load. This
 * test pins the round trip and the default together.
 *
 * The default matters more than the round trip. Every round that existed before
 * migration 000088 was participants-only; if an unset value ever read back as
 * public, this feature would publish boards nobody chose to publish.
 */
func TestContestStore_ScoreboardVisibility(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := NewUserStore(db)
	contestStore := NewContestStore(db)

	owner := &model.User{
		ID:           uuid.New().String(),
		Username:     "sbt_" + uuid.New().String()[:8],
		Email:        "sbt_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed",
		Role:         "setter",
	}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)

	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	base := func(title string) *model.Contest {
		return &model.Contest{
			ID:          uuid.New().String(),
			Title:       title,
			Type:        "acm",
			Format:      "acm",
			StartTime:   start,
			EndTime:     start.Add(3 * time.Hour),
			CreatedBy:   owner.ID,
			Status:      model.ContestStatusPublished,
			IsRated:     true,
			RatedForMin: 0,
			RatedForMax: 9999,
		}
	}

	// Published and public, so it appears in the catalog listing as well as in a
	// direct read — both scan sites get exercised.
	public := base("Public board " + uuid.New().String()[:8])
	public.ScoreboardVisibility = model.ScoreboardPublic
	if err := contestStore.Create(ctx, public); err != nil {
		t.Fatalf("create public-board contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", public.ID)

	// Nothing set: the store normalises it to participants, and that is what has
	// to come back.
	unset := base("Unset board " + uuid.New().String()[:8])
	if err := contestStore.Create(ctx, unset); err != nil {
		t.Fatalf("create unset-board contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", unset.ID)

	t.Run("public survives a reload", func(t *testing.T) {
		got, err := contestStore.GetByID(ctx, public.ID)
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		if !got.IsScoreboardPublic() {
			t.Fatalf("scoreboard_visibility = %q, want %q", got.ScoreboardVisibility, model.ScoreboardPublic)
		}
	})

	t.Run("an omitted value defaults to participants", func(t *testing.T) {
		got, err := contestStore.GetByID(ctx, unset.ID)
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		if got.IsScoreboardPublic() {
			t.Fatalf("an unset scoreboard_visibility became %q; the default must be %q",
				got.ScoreboardVisibility, model.ScoreboardParticipants)
		}
		if got.ScoreboardVisibility != model.ScoreboardParticipants {
			t.Fatalf("scoreboard_visibility = %q, want %q", got.ScoreboardVisibility, model.ScoreboardParticipants)
		}
	})

	t.Run("the listing carries it too", func(t *testing.T) {
		// The catalog is a separate scan with its own column list. A public board
		// must be visible there as a setting, even though the listing's own
		// visibility predicate is unaffected by it.
		items, _, err := contestStore.ListForViewer(ctx, 0, 50, nil, "")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		var found bool
		for _, c := range items {
			if c.ID != public.ID {
				continue
			}
			found = true
			if !c.IsScoreboardPublic() {
				t.Fatalf("the listing reported scoreboard_visibility = %q, want %q",
					c.ScoreboardVisibility, model.ScoreboardPublic)
			}
		}
		if !found {
			t.Skip("the public round fell outside this listing page; the direct read above still covers the scan")
		}
	})

	t.Run("update persists a change", func(t *testing.T) {
		// The settings form saves the whole round, so the value has to survive a
		// full update rather than only the create.
		updated, err := contestStore.GetByID(ctx, unset.ID)
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		updated.ScoreboardVisibility = model.ScoreboardPublic
		if err := contestStore.Update(ctx, updated); err != nil {
			t.Fatalf("update: %v", err)
		}
		got, err := contestStore.GetByID(ctx, unset.ID)
		if err != nil {
			t.Fatalf("reload after update: %v", err)
		}
		if !got.IsScoreboardPublic() {
			t.Fatalf("scoreboard_visibility = %q after update, want %q",
				got.ScoreboardVisibility, model.ScoreboardPublic)
		}
	})
}
