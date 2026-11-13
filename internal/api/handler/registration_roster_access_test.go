package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

/**
 * The registration roster is staff information.
 *
 * GET /contests/{id}/registrations handed every registrant's username to any
 * caller who knew a contest id — the route was mounted outside the authenticated
 * group and the handler asked nothing. On a public round that is the complete
 * competitor list for free; on an invite-only or onsite round it is the list of
 * people who were not even supposed to know it existed.
 *
 * The test drives the real handler against a real database so that it exercises
 * the same path production does — the pre-fix handler returned 200 for all three
 * cases below.
 */
func TestListRegistrationsRefusesNonStaff(t *testing.T) {
	db := openRosterTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := postgres.NewUserStore(db)
	contestStore := postgres.NewContestStore(db)
	regStore := postgres.NewRegistrationStore(db)
	h := NewRegistrationHandler(regStore, contestStore)

	owner := &model.User{
		ID: uuid.New().String(), Username: "roster_" + uuid.New().String()[:8],
		Email:        "roster_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: model.RoleSetter,
	}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	c := &model.Contest{
		ID: uuid.New().String(), Title: "Roster " + uuid.New().String()[:8],
		Type: "acm", Format: "acm", StartTime: start, EndTime: start.Add(2 * time.Hour),
		CreatedBy: owner.ID, Status: model.ContestStatusPublished,
		IsRated: true, RatedForMin: 0, RatedForMax: 9999,
	}
	if err := contestStore.Create(ctx, c); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	// A stranger who is signed in but has no permission on this round.
	stranger := &model.User{
		ID: uuid.New().String(), Username: "stranger_" + uuid.New().String()[:8],
		Email:        "stranger_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: model.RoleUser,
	}
	if err := userStore.Create(ctx, stranger); err != nil {
		t.Fatalf("create stranger: %v", err)
	}
	if err := regStore.Register(ctx, c.ID, stranger.ID); err != nil {
		t.Fatalf("register stranger: %v", err)
	}
	// A second registrant, so a leak would expose more than the caller themselves.
	other := &model.User{
		ID: uuid.New().String(), Username: "other_" + uuid.New().String()[:8],
		Email:        "other_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: model.RoleUser,
	}
	if err := userStore.Create(ctx, other); err != nil {
		t.Fatalf("create other: %v", err)
	}
	if err := regStore.Register(ctx, c.ID, other.ID); err != nil {
		t.Fatalf("register other: %v", err)
	}
	defer func() {
		db.ExecContext(ctx, "DELETE FROM contest_registrations WHERE contest_id = $1", c.ID)
		db.ExecContext(ctx, "DELETE FROM contest_permissions WHERE contest_id = $1", c.ID)
		db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", c.ID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id = ANY($1)", []string{owner.ID, stranger.ID, other.ID})
	}()

	t.Run("anonymous", func(t *testing.T) {
		rec := serveRoster(t, h, c.ID, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous roster read = %d, want 401 (body=%q)", rec.Code, rec.Body.String())
		}
		if names := usernamesIn(rec.Body.Bytes()); len(names) > 0 {
			t.Fatalf("anonymous caller received %d usernames: %v", len(names), names)
		}
	})

	t.Run("signed in but not staff", func(t *testing.T) {
		rec := serveRoster(t, h, c.ID, &auth.Claims{UserID: stranger.ID, Role: model.RoleUser})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("non-staff roster read = %d, want 403 (body=%q)", rec.Code, rec.Body.String())
		}
		if names := usernamesIn(rec.Body.Bytes()); len(names) > 0 {
			t.Fatalf("non-staff caller received %d usernames: %v", len(names), names)
		}
	})

	t.Run("round creator sees the roster", func(t *testing.T) {
		rec := serveRoster(t, h, c.ID, &auth.Claims{UserID: owner.ID, Role: model.RoleSetter})
		if rec.Code != http.StatusOK {
			t.Fatalf("creator roster read = %d, want 200 (body=%q)", rec.Code, rec.Body.String())
		}
		if names := usernamesIn(rec.Body.Bytes()); len(names) != 2 {
			t.Fatalf("creator saw %d usernames, want 2: %v", len(names), names)
		}
	})
}

func serveRoster(t *testing.T, h *RegistrationHandler, contestID string, claims *auth.Claims) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/contests/"+contestID+"/registrations", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", contestID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	if claims != nil {
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))
	}
	rec := httptest.NewRecorder()
	h.ListRegistrations(rec, req)
	return rec
}

// usernamesIn reports how many distinct usernames a response body carries, so a
// refusal can be shown to carry none rather than merely a non-200 status.
func usernamesIn(body []byte) []string {
	var payload struct {
		Data []struct {
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	var out []string
	for _, r := range payload.Data {
		if r.Username != "" {
			out = append(out, r.Username)
		}
	}
	return out
}

// openRosterTestDB mirrors openTestDB in package postgres, which is not visible
// from here. Same DSN contract, same skip semantics when no database is running.
func openRosterTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("AIOJ_TEST_DSN")
	if dsn == "" {
		dsn = "postgres://aioj:aioj_secret@localhost:5432/aioj?sslmode=disable"
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("postgres unavailable (set AIOJ_TEST_DSN or start docker): %v", err)
	}
	return db
}
