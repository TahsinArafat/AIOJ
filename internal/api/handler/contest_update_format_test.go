package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

/**
 * A round's scoring format can be corrected after the fact.
 *
 * Format was accepted on Create and dropped on every Update: the DTO carried it,
 * the store's UPDATE wrote `format=$3`, and the handler simply never assigned
 * c.Format from the request. So a round created with the wrong engine kept it, and
 * the only fix was to recreate the round — losing every submission against it.
 *
 * Three things have to hold, and each is a way this goes wrong quietly:
 *
 *   1. A new format is STORED. Asserted on the row, not the response: the pre-fix
 *      handler answered 200 with the old format still in the database, which is
 *      exactly how the gap survived a passing test suite.
 *   2. format_config follows the format. The configs are not interchangeable — an
 *      acm penalty-per-wrong under an ioi engine has no partial-credit key — so a
 *      stale config against a new format is a round scored with the new engine's
 *      zero values. A format change resets it to that format's default.
 *   3. An unknown format is a 400 naming a reason, not a stored name that
 *      format.Create rejects on every scoreboard render.
 *
 * The "omitted leaves it alone" case matters as much as the change: every other
 * control on this form is saved in the same request, and a settings save that
 * reset the format every time would make the setting unusable.
 */
func TestContestUpdateChangesScoringFormat(t *testing.T) {
	db := openHandlerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := postgres.NewUserStore(db)
	contestStore := postgres.NewContestStore(db)
	h := &ContestHandler{store: contestStore, cache: newScoreboardCache(time.Minute)}

	owner := &model.User{
		ID: uuid.New().String(), Username: "fmt_" + uuid.New().String()[:8],
		Email:        "fmt_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: model.RoleSetter,
	}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	c := &model.Contest{
		ID: uuid.New().String(), Title: "Format " + uuid.New().String()[:8],
		Type: "acm", Format: "acm", StartTime: start, EndTime: start.Add(2 * time.Hour),
		CreatedBy: owner.ID, Status: model.ContestStatusPublished,
		IsRated: true, RatedForMin: 0, RatedForMax: 9999,
	}
	if err := contestStore.Create(ctx, c); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer func() {
		db.ExecContext(ctx, "DELETE FROM contest_permissions WHERE contest_id = $1", c.ID)
		db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", c.ID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)
	}()

	t.Run("a new format is stored", func(t *testing.T) {
		rec := putContest(t, h, c.ID, owner.ID, `{"format":"ioi"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("update = %d, want 200 (body=%q)", rec.Code, rec.Body.String())
		}
		got, err := contestStore.GetByID(ctx, c.ID)
		if err != nil || got == nil {
			t.Fatalf("get: %v", err)
		}
		if got.Format != "ioi" {
			t.Fatalf("format = %q after sending ioi, want ioi — a round's scoring "+
				"could not be corrected without recreating it", got.Format)
		}
	})

	t.Run("the config resets to the new format's default", func(t *testing.T) {
		// Seed a config that is recognisably acm's, then change format. Carrying it
		// across is the failure: the new engine would read keys that are not there.
		got, _ := contestStore.GetByID(ctx, c.ID)
		got.FormatConfig = json.RawMessage(`{"penalty_per_wrong":20,"time_penalty":true}`)
		if err := contestStore.Update(ctx, got); err != nil {
			t.Fatalf("seed config: %v", err)
		}

		rec := putContest(t, h, c.ID, owner.ID, `{"format":"codeforces"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("update = %d, want 200 (body=%q)", rec.Code, rec.Body.String())
		}
		after, _ := contestStore.GetByID(ctx, c.ID)
		if after.Format != "codeforces" {
			t.Fatalf("format = %q, want codeforces", after.Format)
		}
		var cfg map[string]any
		if err := json.Unmarshal(after.FormatConfig, &cfg); err != nil {
			t.Fatalf("stored format_config is not JSON: %v (%s)", err, after.FormatConfig)
		}
		// The point is not that a specific key exists — it is that the acm config did
		// not survive into a format that has no use for it. Asserting the absence of
		// acm's own key is what catches a carry-across.
		if _, carried := cfg["penalty_per_wrong"]; carried {
			t.Fatalf("format_config still carries acm's penalty_per_wrong after "+
				"switching to codeforces: %s", after.FormatConfig)
		}
	})

	t.Run("an unknown format is a 400 that names the reason", func(t *testing.T) {
		before, _ := contestStore.GetByID(ctx, c.ID)
		rec := putContest(t, h, c.ID, owner.ID, `{"format":"nonsense"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("update = %d, want 400 — an unknown format must be refused, "+
				"not stored for the scoreboard to fail on (body=%q)", rec.Code, rec.Body.String())
		}
		after, _ := contestStore.GetByID(ctx, c.ID)
		if after.Format != before.Format {
			t.Fatalf("format = %q after a rejected save, want it unchanged at %q",
				after.Format, before.Format)
		}
	})

	t.Run("an omitted format leaves it alone", func(t *testing.T) {
		// Every other control on the form saves in the same request, so a save that
		// did not mention the format must not reset it.
		before, _ := contestStore.GetByID(ctx, c.ID)
		rec := putContest(t, h, c.ID, owner.ID, `{"title":"renamed only"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("update = %d, want 200 (body=%q)", rec.Code, rec.Body.String())
		}
		after, _ := contestStore.GetByID(ctx, c.ID)
		if after.Format != before.Format {
			t.Fatalf("format = %q after an unrelated save, want %q", after.Format, before.Format)
		}
		if after.Title != "renamed only" {
			t.Fatalf("title = %q, want renamed only", after.Title)
		}
	})
}
