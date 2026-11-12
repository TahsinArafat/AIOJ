package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// errContestPasswordTooLong is the only client-side rule on a contest password.
// Length is the one property bcrypt cannot absorb: past 72 bytes it silently
// truncates, so two different long passwords would unlock the same round.
var errContestPasswordTooLong = errors.New("contest password must be at most 72 bytes")

// hashContestPassword hashes a setter-supplied password, or returns "" for nil to
// mean "this round has no password".
//
// Reusing auth.HashPassword keeps contests on the same bcrypt cost as accounts, and
// means the hash format the migration backfill produces (pgcrypto's
// crypt(..., gen_salt('bf', 12))) is verified by the same function at runtime.
func hashContestPassword(password *string) (string, error) {
	if password == nil || *password == "" {
		return "", nil
	}
	if len(*password) > 72 {
		return "", errContestPasswordTooLong
	}
	return auth.HashPassword(*password)
}

// ContestUnlockHandler issues the entitlement a contestant needs to open a
// password-protected round.
//
// It lives in its own handler, with its own routes, because unlocking is a
// credential check — not a read of contest data — and credential checks need
// different treatment: rate limiting, uniform failure, and no participation in the
// read path's caching.
type ContestUnlockHandler struct {
	contests contestUnlockStore
	grants   store.ContestGrantStore
}

func NewContestUnlockHandler(cs contestUnlockStore, gs store.ContestGrantStore) *ContestUnlockHandler {
	return &ContestUnlockHandler{contests: cs, grants: gs}
}

// contestUnlockStore is the narrow slice of the contest store an unlock needs:
// the round, so it can compare the password, and nothing else.
type contestUnlockStore interface {
	GetByID(ctx context.Context, id string) (*model.Contest, error)
}

// Unlock exchanges the round's password for a grant.
//
// POST /api/contests/{id}/unlock  {"password": "..."}
//
//	204 — granted (idempotent)
//	400 — the round has no password to unlock
//	401 — not signed in
//	403 — wrong password
//	404 — no such contest
func (h *ContestUnlockHandler) Unlock(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contest, err := h.contests.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	// A round with no password has nothing to unlock. Allowing it would hand out a
	// grant row for a public contest and, worse, teach a client that
	// "204 means I am in" is meaningful on any round.
	if contest.PasswordHash == "" {
		http.Error(w, "this contest has no password", http.StatusBadRequest)
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.Password == "" {
		http.Error(w, "password required", http.StatusBadRequest)
		return
	}

	if !auth.CheckPassword(body.Password, contest.PasswordHash) {
		// One uniform message, and the bcrypt comparison above already dominates
		// the response time, so a wrong guess reveals nothing about the round. The
		// route is wrapped in StrictAuth, which is what keeps this from being an
		// offline-speed oracle on a short password.
		http.Error(w, "incorrect password", http.StatusForbidden)
		return
	}

	if err := h.grants.Grant(r.Context(), contest.ID, claims.UserID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
