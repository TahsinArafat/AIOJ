package handler

import (
	"bytes"
	"context"
	"encoding/json"
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
 * Only an admin decides whether a round is rated.
 *
 * is_rated is not a setting like `listed`: it decides whether the round moves
 * every entrant's rating on the global leaderboard. Until this, Create and Update
 * both wrote it straight from the request, so a setter could announce a rated
 * round — or withdraw one whose ratings had already been paid out — with no
 * review. Publishing was gated the same way (initialContestStatus); rating was the
 * half that was missed, and the half with a leaderboard attached.
 *
 * An author now records proposed_rated; the reviewer resolves it on publish. Both
 * halves are asserted here, on the row rather than the response: the failure mode
 * is a 200 with the wrong column changed, which a status code cannot distinguish.
 */

// ratingApprovalPut is putContest with a chosen role: the author case is the one
// under test, so the role cannot be fixed at model.RoleSetter.
func ratingApprovalPut(t *testing.T, h *ContestHandler, contestID, userID, role, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/contests/"+contestID, bytes.NewBufferString(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", contestID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey,
		&auth.Claims{UserID: userID, Role: role}))
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	return rec
}

// ratingApprovalReview is the admin verdict endpoint, which is the only thing that
// publishes a round and therefore the only thing that resolves a rating proposal.
func ratingApprovalReview(t *testing.T, h *ContestHandler, contestID, adminID, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/contests/"+contestID+"/review", bytes.NewBufferString(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", contestID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey,
		&auth.Claims{UserID: adminID, Role: model.RoleAdmin}))
	rec := httptest.NewRecorder()
	h.ReviewContest(rec, req)
	return rec
}

// ratingApprovalFixture is a round with a manager who authored it, plus the admin who
// reviews it.
type ratingApprovalFixture struct {
	store   *postgres.ContestStore
	handler *ContestHandler
	author  *model.User
	admin   *model.User
	round   *model.Contest
}

func newRatingApprovalFixture(t *testing.T, status string) *ratingApprovalFixture {
	t.Helper()
	db := testdb.Require(t)
	ctx := context.Background()

	userStore := postgres.NewUserStore(db)
	store := postgres.NewContestStore(db)

	author := &model.User{
		ID: uuid.New().String(), Username: "ra_" + uuid.New().String()[:8],
		Email: "ra_" + uuid.New().String()[:8] + "@example.com", PasswordHash: "hashed",
		Role: model.RoleSetter,
	}
	admin := &model.User{
		ID: uuid.New().String(), Username: "aa_" + uuid.New().String()[:8],
		Email: "aa_" + uuid.New().String()[:8] + "@example.com", PasswordHash: "hashed",
		Role: model.RoleAdmin,
	}
	for _, u := range []*model.User{author, admin} {
		if err := userStore.Create(ctx, u); err != nil {
			t.Fatalf("create user: %v", err)
		}
	}

	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	round := &model.Contest{
		ID: uuid.New().String(), Title: "Rating " + uuid.New().String()[:8],
		Type: "acm", Format: "acm", StartTime: start, EndTime: start.Add(2 * time.Hour),
		CreatedBy: author.ID, Status: status,
		IsRated: true, ProposedRated: true, RatedForMin: 0, RatedForMax: 9999,
	}
	if err := store.Create(ctx, round); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, "DELETE FROM contest_permissions WHERE contest_id = $1", round.ID)
		db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", round.ID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id = ANY($1)", []string{author.ID, admin.ID})
	})

	return &ratingApprovalFixture{
		store:   store,
		handler: &ContestHandler{store: store, cache: newScoreboardCache(time.Minute)},
		author:  author,
		admin:   admin,
		round:   round,
	}
}

// An author's declaration is a proposal. It must not reach is_rated, or the review
// queue would be decorative for the one field with a leaderboard attached.
func TestContestRatingProposalDoesNotRateTheRound(t *testing.T) {
	f := newRatingApprovalFixture(t, model.ContestStatusDraft)
	ctx := context.Background()

	rec := ratingApprovalPut(t, f.handler, f.round.ID, f.author.ID, model.RoleSetter, `{"is_rated":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update = %d, want 200 — the same PUT carries every other settings "+
			"field, so refusing it would block an unrelated rename (body=%q)",
			rec.Code, rec.Body.String())
	}

	got, err := f.store.GetByID(ctx, f.round.ID)
	if err != nil || got == nil {
		t.Fatalf("get: %v", err)
	}
	if !got.IsRated {
		t.Fatal("an author's is_rated=false reached is_rated; only a published verdict may rate a round")
	}
	if got.ProposedRated {
		t.Fatal("proposed_rated was not recorded — the reviewer would have nothing to decide")
	}
}

// Publishing resolves the proposal, which is the whole point: the reviewer approves
// the round and its rating in one action.
func TestPublishResolvesTheRatingProposal(t *testing.T) {
	f := newRatingApprovalFixture(t, model.ContestStatusPending)
	ctx := context.Background()

	if rec := ratingApprovalPut(t, f.handler, f.round.ID, f.author.ID, model.RoleSetter, `{"is_rated":false}`); rec.Code != http.StatusOK {
		t.Fatalf("propose: %d body=%q", rec.Code, rec.Body.String())
	}
	if rec := ratingApprovalReview(t, f.handler, f.round.ID, f.admin.ID, `{"status":"published"}`); rec.Code != http.StatusOK {
		t.Fatalf("review: %d body=%q", rec.Code, rec.Body.String())
	}

	got, _ := f.store.GetByID(ctx, f.round.ID)
	if got.Status != model.ContestStatusPublished {
		t.Fatalf("status = %q, want published", got.Status)
	}
	if got.IsRated {
		t.Fatal("the round was published as rated although its author proposed unrated")
	}
}

// The reviewer's own answer overrides the proposal, in the same verdict — so a
// round is never rated first and corrected afterwards.
func TestReviewVerdictOverridesTheRatingProposal(t *testing.T) {
	f := newRatingApprovalFixture(t, model.ContestStatusPending)
	ctx := context.Background()

	if rec := ratingApprovalReview(t, f.handler, f.round.ID, f.admin.ID,
		`{"status":"published","is_rated":false}`); rec.Code != http.StatusOK {
		t.Fatalf("review: %d body=%q", rec.Code, rec.Body.String())
	}

	got, _ := f.store.GetByID(ctx, f.round.ID)
	if got.IsRated {
		t.Fatal("the reviewer's is_rated=false did not reach the round on publish")
	}
}

// Sending a round back must not change its rating: a rejection is not a rating
// decision, and the same proposal is approved later.
func TestRejectionLeavesTheRatingAlone(t *testing.T) {
	f := newRatingApprovalFixture(t, model.ContestStatusPending)
	ctx := context.Background()

	if rec := ratingApprovalReview(t, f.handler, f.round.ID, f.admin.ID, `{"status":"rejected","note":"fix it"}`); rec.Code != http.StatusOK {
		t.Fatalf("review: %d body=%q", rec.Code, rec.Body.String())
	}
	got, _ := f.store.GetByID(ctx, f.round.ID)
	if !got.IsRated {
		t.Fatal("a rejection unrated the round; only publishing resolves the proposal")
	}
}

// The rating window stays the author's: it bounds which entrants of an already
// rated round count, so it changes nothing about whether the round is rated.
func TestRatingWindowStaysSettableByTheAuthor(t *testing.T) {
	f := newRatingApprovalFixture(t, model.ContestStatusDraft)
	ctx := context.Background()

	rec := ratingApprovalPut(t, f.handler, f.round.ID, f.author.ID, model.RoleSetter,
		`{"rated_for_min":1200,"rated_for_max":2400}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update = %d, want 200 (body=%q)", rec.Code, rec.Body.String())
	}
	got, _ := f.store.GetByID(ctx, f.round.ID)
	if got.RatedForMin != 1200 || got.RatedForMax != 2400 {
		t.Fatalf("window = [%d, %d], want [1200, 2400]", got.RatedForMin, got.RatedForMax)
	}
}

// Create is the other door: a setter's is_rated=false must not produce an unrated
// round, or gating Update alone would be decorative.
func TestContestCreateRecordsSetterRatingAsProposal(t *testing.T) {
	db := testdb.Require(t)
	ctx := context.Background()
	userStore := postgres.NewUserStore(db)
	store := postgres.NewContestStore(db)

	setter := &model.User{
		ID: uuid.New().String(), Username: "cs_" + uuid.New().String()[:8],
		Email: "cs_" + uuid.New().String()[:8] + "@example.com", PasswordHash: "hashed",
		Role: model.RoleSetter,
	}
	if err := userStore.Create(ctx, setter); err != nil {
		t.Fatalf("create setter: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", setter.ID)

	h := &ContestHandler{store: store, cache: newScoreboardCache(time.Minute)}
	req := httptest.NewRequest(http.MethodPost, "/api/contests", bytes.NewBufferString(`{"title":"Proposed","is_rated":false}`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey,
		&auth.Claims{UserID: setter.ID, Role: model.RoleSetter}))
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d, want 201 (body=%q)", rec.Code, rec.Body.String())
	}

	var created model.Contest
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	defer func() {
		db.ExecContext(ctx, "DELETE FROM contest_permissions WHERE contest_id = $1", created.ID)
		db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", created.ID)
	}()

	got, err := store.GetByID(ctx, created.ID)
	if err != nil || got == nil {
		t.Fatalf("get: %v", err)
	}
	if !got.IsRated {
		t.Fatal("a setter created an unrated round outright; the reviewer decides that")
	}
	if got.ProposedRated {
		t.Fatal("the proposal was not recorded, so the reviewer has nothing to decide")
	}
	if got.Status == model.ContestStatusPublished {
		t.Fatalf("status = %q: a setter's round must not start published", got.Status)
	}
}
