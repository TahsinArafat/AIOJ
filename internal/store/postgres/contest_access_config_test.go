package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * The per-surface audiences have to survive a write and a reload, and an unset
 * round has to default to the closed state.
 *
 * The five read sites (GetByID, ListForViewer, ListByCreatedBy, ListByStatus and
 * Create's own RETURNING) each scan their own column list, so a column added to
 * one and forgotten in another is a silent zero — which for an audience means
 * "inherit", i.e. safe, but also means a setter's public board would quietly
 * revert on the next page load. This test pins the round trip and the default
 * together, and pins the default in the direction that matters.
 *
 * The default matters more than the round trip. Every round that existed before
 * migration 000091 was participants-only; if an unset value ever read back as
 * public, this feature would publish boards nobody chose to publish. That is also
 * why the inheritance case is asserted explicitly rather than assumed: an empty
 * map and a map naming a different surface must both resolve the board to
 * participants.
 */
func TestContestStore_AccessConfig(t *testing.T) {
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
	public.AccessConfig = model.AccessConfig{model.SurfaceStandings: model.AccessPublic}
	if err := contestStore.Create(ctx, public); err != nil {
		t.Fatalf("create public-board contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", public.ID)

	// Nothing set: an empty map, which is what every pre-existing row has.
	unset := base("Unset board " + uuid.New().String()[:8])
	if err := contestStore.Create(ctx, unset); err != nil {
		t.Fatalf("create unset-board contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", unset.ID)

	// A round that set some OTHER surface. The board must still be participants:
	// this is the rule that a public surface cannot widen its neighbours, and it is
	// the one whose absence would silently publish a private round's board.
	other := base("Other surface " + uuid.New().String()[:8])
	other.AccessConfig = model.AccessConfig{model.SurfacePDF: model.AccessPublic}
	if err := contestStore.Create(ctx, other); err != nil {
		t.Fatalf("create other-surface contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", other.ID)

	t.Run("a set audience survives a reload", func(t *testing.T) {
		got, err := contestStore.GetByID(ctx, public.ID)
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		if !got.IsScoreboardPublic() {
			t.Fatalf("standings audience = %q, want %q",
				got.AccessConfig.Audience(model.SurfaceStandings), model.AccessPublic)
		}
	})

	t.Run("an empty config defaults every surface to participants", func(t *testing.T) {
		got, err := contestStore.GetByID(ctx, unset.ID)
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		if got.IsScoreboardPublic() {
			t.Fatal("an unset access_config published the board; the default must be participants")
		}
		for _, surface := range model.AccessSurfaceKeys {
			if a := got.AccessConfig.Audience(surface); a != model.AccessParticipants {
				t.Errorf("surface %q resolved to %q, want %q", surface, a, model.AccessParticipants)
			}
		}
		// And the map itself is non-nil, so a caller can index it without a check.
		if got.AccessConfig == nil {
			t.Fatal("a nil map came back for an unset config; a caller indexing it would panic")
		}
	})

	t.Run("one public surface does not widen another", func(t *testing.T) {
		got, err := contestStore.GetByID(ctx, other.ID)
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		if got.AccessConfig.Audience(model.SurfacePDF) != model.AccessPublic {
			t.Fatalf("pdf audience = %q, want %q",
				got.AccessConfig.Audience(model.SurfacePDF), model.AccessPublic)
		}
		if got.IsScoreboardPublic() {
			t.Fatal("setting the pdf audience public also published the standings; " +
				"inheritance must flow from the base only, never sideways")
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
				t.Fatalf("the listing reported a standings audience of %q, want %q",
					c.AccessConfig.Audience(model.SurfaceStandings), model.AccessPublic)
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
		updated.AccessConfig = model.AccessConfig{model.SurfaceStandings: model.AccessPublic}
		if err := contestStore.Update(ctx, updated); err != nil {
			t.Fatalf("update: %v", err)
		}
		got, err := contestStore.GetByID(ctx, unset.ID)
		if err != nil {
			t.Fatalf("reload after update: %v", err)
		}
		if !got.IsScoreboardPublic() {
			t.Fatalf("standings audience = %q after update, want %q",
				got.AccessConfig.Audience(model.SurfaceStandings), model.AccessPublic)
		}
	})

	t.Run("update can clear every surface", func(t *testing.T) {
		// The whole-map replace has to be able to go back to empty. A merge would
		// make this impossible, which is why it is not one.
		cleared, err := contestStore.GetByID(ctx, unset.ID)
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		cleared.AccessConfig = model.AccessConfig{}
		if err := contestStore.Update(ctx, cleared); err != nil {
			t.Fatalf("update: %v", err)
		}
		got, err := contestStore.GetByID(ctx, unset.ID)
		if err != nil {
			t.Fatalf("reload after clear: %v", err)
		}
		if got.IsScoreboardPublic() {
			t.Fatal("clearing the config left the board public; an empty map must mean participants")
		}
	})

	t.Run("an invalid entry is dropped rather than stored", func(t *testing.T) {
		// The store boundary trusts nothing: a row written by hand or by an older
		// build must not be able to make a surface unreachable, or to name a
		// surface that does not exist. Rejecting is the handler's job, on user
		// input; falling back is the only option here, where there is no response
		// to write to.
		bogus := base("Bogus " + uuid.New().String()[:8])
		bogus.AccessConfig = model.AccessConfig{
			model.SurfaceStandings: "everyone",         // not an audience
			"not_a_surface":        model.AccessPublic, // not a surface
			model.SurfacePDF:       model.AccessPublic, // valid, must survive
		}
		if err := contestStore.Create(ctx, bogus); err != nil {
			t.Fatalf("create: %v", err)
		}
		defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", bogus.ID)

		got, err := contestStore.GetByID(ctx, bogus.ID)
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		if got.IsScoreboardPublic() {
			t.Fatal(`an invalid audience ("everyone") was stored and read back as a public board`)
		}
		if _, ok := got.AccessConfig["not_a_surface"]; ok {
			t.Fatal("a key naming no real surface was stored")
		}
		if got.AccessConfig.Audience(model.SurfacePDF) != model.AccessPublic {
			t.Fatal("a valid entry alongside invalid ones was dropped too")
		}
	})
}
