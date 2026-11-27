package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
	"github.com/tahsinarafat/aioj/internal/testdb"
)

/**
 * A settings save must be able to remove a limit it once set.
 *
 * max_participants, registration_deadline and freeze_time are nullable, and the
 * handler read each through `if req.X != nil`, which cannot tell "clear it" from
 * "don't touch it" — a JSON null and an absent key both produce a nil pointer. The
 * Settings tab expresses "no cap" and "no freeze" by sending null, so a cap or a
 * freeze could be set once and never removed. The handler now decodes the set of
 * keys as well as the struct.
 *
 * The assertion is on the stored row, not on the response: the pre-fix handler
 * returned 200 with the old value still in the database, which is exactly why this
 * went unnoticed.
 */
func TestContestUpdateClearsNullableFieldsWhenSentAsNull(t *testing.T) {
	db := testdb.Require(t)

	ctx := context.Background()
	userStore := postgres.NewUserStore(db)
	contestStore := postgres.NewContestStore(db)
	h := &ContestHandler{store: contestStore, cache: newScoreboardCache(time.Minute)}

	owner := &model.User{
		ID: uuid.New().String(), Username: "hnd_" + uuid.New().String()[:8],
		Email:        "hnd_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: model.RoleSetter,
	}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	cap := 25
	deadline := start.Add(-time.Minute)
	freeze := start.Add(30 * time.Minute)
	c := &model.Contest{
		ID: uuid.New().String(), Title: "Handler " + uuid.New().String()[:8],
		Type: "acm", Format: "acm", StartTime: start, EndTime: start.Add(2 * time.Hour),
		FreezeTime: &freeze,
		CreatedBy:  owner.ID, Status: model.ContestStatusPublished,
		IsRated: true, RatedForMin: 0, RatedForMax: 9999,
		RegistrationRequired: true, MaxParticipants: &cap, RegistrationDeadline: &deadline,
	}
	if err := contestStore.Create(ctx, c); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer func() {
		db.ExecContext(ctx, "DELETE FROM contest_permissions WHERE contest_id = $1", c.ID)
		db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", c.ID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)
	}()

	t.Run("null clears", func(t *testing.T) {
		rec := putContest(t, h, c.ID, owner.ID, `{"max_participants":null,"registration_deadline":null,"freeze_time":null}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("update = %d, want 200 (body=%q)", rec.Code, rec.Body.String())
		}
		got, err := contestStore.GetByID(ctx, c.ID)
		if err != nil || got == nil {
			t.Fatalf("get: %v", err)
		}
		if got.MaxParticipants != nil {
			t.Fatalf("max_participants = %d after sending null, want NULL — a cap that cannot be cleared", *got.MaxParticipants)
		}
		if got.RegistrationDeadline != nil {
			t.Fatalf("registration_deadline = %v after sending null, want NULL", *got.RegistrationDeadline)
		}
		if got.FreezeTime != nil {
			t.Fatalf("freeze_time = %v after sending null, want NULL — a freeze that cannot be cleared", *got.FreezeTime)
		}
	})

	t.Run("omitted leaves the column alone", func(t *testing.T) {
		// Re-set a cap and a freeze, then save an unrelated field and confirm
		// both survive.
		again := 99
		got, _ := contestStore.GetByID(ctx, c.ID)
		got.MaxParticipants = &again
		refreeze := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
		got.FreezeTime = &refreeze
		if err := contestStore.Update(ctx, got); err != nil {
			t.Fatalf("seed cap: %v", err)
		}
		rec := putContest(t, h, c.ID, owner.ID, `{"title":"renamed"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("update = %d, want 200 (body=%q)", rec.Code, rec.Body.String())
		}
		after, _ := contestStore.GetByID(ctx, c.ID)
		if after.MaxParticipants == nil || *after.MaxParticipants != 99 {
			t.Fatalf("max_participants = %v after an unrelated save, want 99 (an omitted field must not clear it)", after.MaxParticipants)
		}
		if after.FreezeTime == nil {
			t.Fatalf("freeze_time was cleared by an unrelated save (an omitted field must not clear it)")
		}
		if after.Title != "renamed" {
			t.Fatalf("title = %q, want renamed", after.Title)
		}
	})
}

func putContest(t *testing.T, h *ContestHandler, contestID, ownerID, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/contests/"+contestID, bytes.NewBufferString(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", contestID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey,
		&auth.Claims{UserID: ownerID, Role: model.RoleSetter}))
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	return rec
}
