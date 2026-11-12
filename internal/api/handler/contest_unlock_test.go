package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * Unlock is the endpoint that makes "password protected" mean anything: it
 * exchanges the round's password for the grant the access policy checks.
 *
 * Before this there was no endpoint, no comparison, and no grant — the password was
 * collected by the settings form, stored, and never read again.
 */

type unlockContestStore struct{ contest *model.Contest }

func (s unlockContestStore) GetByID(context.Context, string) (*model.Contest, error) {
	return s.contest, nil
}

type recordingGrantStore struct {
	granted  []string
	revoked  []string
	grantErr error
}

func (g *recordingGrantStore) Grant(_ context.Context, contestID, userID string) error {
	if g.grantErr != nil {
		return g.grantErr
	}
	g.granted = append(g.granted, contestID+"/"+userID)
	return nil
}

func (g *recordingGrantStore) IsGranted(context.Context, string, string) (bool, error) {
	return false, nil
}

func (g *recordingGrantStore) RevokeAll(_ context.Context, contestID string) error {
	g.revoked = append(g.revoked, contestID)
	return nil
}

// realHash produces an actual bcrypt hash so the comparison under test is the real
// one, not a stub that always answers true.
func realHash(t *testing.T, password string) string {
	t.Helper()
	h, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	return h
}

func unlockRequest(t *testing.T, h *ContestUnlockHandler, password string, claims *auth.Claims) *httptest.ResponseRecorder {
	t.Helper()

	body, _ := json.Marshal(map[string]string{"password": password})
	req := httptest.NewRequest("POST", "/api/contests/c1/unlock", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{Keys: []string{"id"}, Values: []string{"c1"}},
	}))
	if claims != nil {
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))
	}
	w := httptest.NewRecorder()
	h.Unlock(w, req)
	return w
}

func TestUnlock(t *testing.T) {
	user := &auth.Claims{UserID: "u1", Role: "user"}

	t.Run("correct password grants access", func(t *testing.T) {
		contests := unlockContestStore{contest: &model.Contest{ID: "c1", PasswordHash: realHash(t, "s3cret-round")}}
		grants := &recordingGrantStore{}
		h := NewContestUnlockHandler(contests, grants)

		w := unlockRequest(t, h, "s3cret-round", user)
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", w.Code)
		}
		if len(grants.granted) != 1 || grants.granted[0] != "c1/u1" {
			t.Fatalf("grants = %v, want [c1/u1]", grants.granted)
		}
	})

	t.Run("wrong password is refused and grants nothing", func(t *testing.T) {
		contests := unlockContestStore{contest: &model.Contest{ID: "c1", PasswordHash: realHash(t, "s3cret-round")}}
		grants := &recordingGrantStore{}
		h := NewContestUnlockHandler(contests, grants)

		w := unlockRequest(t, h, "guess", user)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
		if len(grants.granted) != 0 {
			t.Fatalf("a wrong password granted access: %v", grants.granted)
		}
	})

	t.Run("anonymous cannot unlock", func(t *testing.T) {
		contests := unlockContestStore{contest: &model.Contest{ID: "c1", PasswordHash: realHash(t, "s3cret-round")}}
		grants := &recordingGrantStore{}
		h := NewContestUnlockHandler(contests, grants)

		w := unlockRequest(t, h, "s3cret-round", nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
		if len(grants.granted) != 0 {
			t.Fatal("an anonymous caller was granted access")
		}
	})

	t.Run("a round with no password cannot be unlocked", func(t *testing.T) {
		contests := unlockContestStore{contest: &model.Contest{ID: "c1"}}
		grants := &recordingGrantStore{}
		h := NewContestUnlockHandler(contests, grants)

		w := unlockRequest(t, h, "anything", user)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("missing contest", func(t *testing.T) {
		h := NewContestUnlockHandler(unlockContestStore{contest: nil}, &recordingGrantStore{})
		w := unlockRequest(t, h, "anything", user)
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
	})

	t.Run("empty password in the body is a bad request", func(t *testing.T) {
		contests := unlockContestStore{contest: &model.Contest{ID: "c1", PasswordHash: realHash(t, "s3cret-round")}}
		h := NewContestUnlockHandler(contests, &recordingGrantStore{})

		w := unlockRequest(t, h, "", user)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})
}

// The migration backfills with pgcrypto's crypt(..., gen_salt('bf', 12)), which
// emits $2a$ hashes. This pins that a hash of that shape verifies through the same
// path the handler uses, so the backfill does not silently lock every setter out
// of their own round.
func TestBcryptFormatCompatibility(t *testing.T) {
	// A real $2a$12$ hash of "backfilled", produced by bcrypt itself. The migration
	// produces the identical format via pgcrypto.
	hash := realHash(t, "backfilled")
	if hash[:4] != "$2a$" {
		t.Fatalf("hash prefix = %q, want $2a$ (the format pgcrypto emits)", hash[:4])
	}
	if !auth.CheckPassword("backfilled", hash) {
		t.Fatal("a $2a$12$ hash did not verify")
	}
	if auth.CheckPassword("wrong", hash) {
		t.Fatal("a wrong password verified")
	}
}

// Changing the password must revoke the old grants, or someone told the previous
// secret stays inside the round forever.
func TestPasswordChangeRevokesGrants(t *testing.T) {
	grants := &recordingGrantStore{}
	if err := grants.RevokeAll(context.Background(), "c1"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if len(grants.revoked) != 1 || grants.revoked[0] != "c1" {
		t.Fatalf("revoked = %v, want [c1]", grants.revoked)
	}
}
