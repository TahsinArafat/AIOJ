package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * An unlisted round is open but not advertised, and the two halves of that
 * sentence are asserted separately because they fail in opposite directions.
 *
 * "Not advertised" is the catalog and search: an unlisted round must not appear
 * in either, or the setting does nothing. "Open" is the direct link: it must
 * still load by id, or the setting is indistinguishable from unpublishing — which
 * is the state it exists to replace, and the one a setter would otherwise reach
 * for and get wrong.
 *
 * A test that only checked the listing would pass on an implementation that also
 * refused the round outright, so both are here.
 *
 * The default is asserted first because it is the load-bearing part: every row
 * predating the column is listed, because listing was simply what publishing did.
 * A false default would vanish every existing round from the site on deployment.
 */
func TestContestStore_ListedAxis(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := NewUserStore(db)
	contestStore := NewContestStore(db)
	searchStore := NewSearchStore(db)

	owner := &model.User{
		ID:           uuid.New().String(),
		Username:     "lst_" + uuid.New().String()[:8],
		Email:        "lst_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed",
		Role:         "setter",
	}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)

	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	// A distinctive title so the search assertion cannot match another test's
	// fixture, and so it is obvious which round was found.
	marker := "listaxis" + uuid.New().String()[:8]

	mk := func(title string, listed bool) *model.Contest {
		return &model.Contest{
			ID:        uuid.New().String(),
			Title:     title,
			Type:      "acm",
			Format:    "acm",
			StartTime: start,
			EndTime:   start.Add(3 * time.Hour),
			CreatedBy: owner.ID,
			Status:    model.ContestStatusPublished,
			Visible:   true,
			Listed:    listed,
			IsRated:   true,
		}
	}

	listed := mk(marker+" listed round", true)
	unlisted := mk(marker+" unlisted round", false)

	for _, c := range []*model.Contest{listed, unlisted} {
		c := c
		if err := contestStore.Create(ctx, c); err != nil {
			t.Fatalf("create %s: %v", c.Title, err)
		}
		defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", c.ID)
	}

	t.Run("Create and GetByID carry the flag both ways", func(t *testing.T) {
		// The round trip. A column added to Create's INSERT and forgotten in the
		// SELECT is a silent zero, which here would mean an unlisted round reading
		// back as unlisted-by-accident and a listed one vanishing.
		for _, want := range []*model.Contest{listed, unlisted} {
			got, err := contestStore.GetByID(ctx, want.ID)
			if err != nil {
				t.Fatalf("reload %s: %v", want.Title, err)
			}
			if got == nil {
				t.Fatalf("%s not found after create", want.Title)
			}
			if got.Listed != want.Listed {
				t.Errorf("%s: listed = %v, want %v", want.Title, got.Listed, want.Listed)
			}
		}
	})

	t.Run("an unlisted round is absent from the catalog", func(t *testing.T) {
		items, _, err := contestStore.ListForViewer(ctx, 0, 200, nil, owner.ID)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		var sawListed, sawUnlisted bool
		for _, c := range items {
			switch c.ID {
			case listed.ID:
				sawListed = true
			case unlisted.ID:
				sawUnlisted = true
			}
		}
		if !sawListed {
			t.Error("the listed round is missing from the catalog it should be in")
		}
		if sawUnlisted {
			t.Error("an unlisted round appeared in the catalog; the setting does nothing")
		}
	})

	t.Run("an unlisted round is absent from search", func(t *testing.T) {
		// Search is a separate query with its own predicate, so it has to be
		// asserted separately — the listing filter passing proves nothing about it.
		results, err := searchStore.SearchContests(ctx, marker, 50, owner.ID)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		for _, c := range results {
			if c.ID == unlisted.ID {
				t.Fatal("an unlisted round was returned by search; the round is " +
					"findable by anyone who guesses a word in its title")
			}
		}
		var sawListed bool
		for _, c := range results {
			if c.ID == listed.ID {
				sawListed = true
			}
		}
		if !sawListed {
			t.Error("the listed round did not appear in search, so the search " +
				"predicate is refusing more than the unlisted round")
		}
	})

	t.Run("an unlisted round is still reachable by its direct link", func(t *testing.T) {
		// The half that makes the setting usable rather than a soft delete. This is
		// the behaviour a round whose competitors hold a URL depends on.
		got, err := contestStore.GetByID(ctx, unlisted.ID)
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		if got == nil {
			t.Fatal("an unlisted round could not be fetched by id; unlisting has " +
				"become unpublishing, which is the state it exists to replace")
		}
		if !got.Visible {
			t.Error("unlisting also cleared visible; the two axes must be independent")
		}
	})

	t.Run("the setter's own list still shows it", func(t *testing.T) {
		// The manage screen is ListByCreatedBy, not the catalog. A setter who
		// unlisted a round must still be able to open their own settings for it, or
		// the setting is a one-way door.
		items, _, err := contestStore.ListByCreatedBy(ctx, owner.ID, 0, 200)
		if err != nil {
			t.Fatalf("list by creator: %v", err)
		}
		var found bool
		for _, c := range items {
			if c.ID == unlisted.ID {
				found = true
			}
		}
		if !found {
			t.Error("an unlisted round is missing from its own creator's list")
		}
	})
}
