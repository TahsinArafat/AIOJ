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
)

/**
 * Voiding a submission is an accusation: it removes an attempt from a public
 * leaderboard. Two things have to hold, and the role policy alone provides
 * neither of them:
 *
 *  1. Only the people accountable for THIS submission may void it — the
 *     problem's owner/co-author, or the round's manager/judge. A setter who
 *     merely holds the role, or an admin of some unrelated part of the site,
 *     must not be able to remove a stranger's accepted run.
 *  2. A submission may not be voided twice, and not while the judge still owns
 *     it.
 *
 * These drive the real handler against a real database, so the permission query
 * and the store write both run for real. The role policy is enforced by
 * middleware on the route (see TestSkipRouteRequiresTheFeatureGate) and is
 * deliberately NOT exercised here — this test is about half 2 of the check.
 */
func TestSkipSubmissionAuthorization(t *testing.T) {
	db := openRosterTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := postgres.NewUserStore(db)
	problemStore := postgres.NewProblemStore(db)
	contestStore := postgres.NewContestStore(db)
	subStore := postgres.NewSubmissionStore(db)
	h := NewSubmissionHandler(subStore, problemStore, contestStore, nil, nil, nil, "", nil, userStore)

	newUser := func(role string) *model.User {
		u := &model.User{
			ID: uuid.New().String(), Username: "skip_" + uuid.New().String()[:8],
			Email: uuid.New().String() + "@example.com", PasswordHash: "hashed", Role: role,
		}
		if err := userStore.Create(ctx, u); err != nil {
			t.Fatalf("create user %s: %v", role, err)
		}
		return u
	}

	owner := newUser(model.RoleSetter)      // owns the problem
	roundManager := newUser(model.RoleUser) // runs the round, nothing to do with the problem's authoring
	stranger := newUser(model.RoleSetter)   // a setter, but accountable for neither
	solver := newUser(model.RoleUser)

	problemID := uuid.New().String()
	if err := problemStore.Create(ctx, &model.Problem{
		ID: problemID, Slug: "skip-" + uuid.New().String()[:8], Title: "Skip",
		Description: "d", TimeLimit: 1000, MemoryLimit: 262144, Difficulty: "easy",
		Tags: []string{}, CreatedBy: owner.ID,
		Status: model.ProblemStatusPublished, Visible: true,
	}); err != nil {
		t.Fatalf("create problem: %v", err)
	}

	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	contest := &model.Contest{
		ID: uuid.New().String(), Title: "Skip " + uuid.New().String()[:8],
		Type: "acm", Format: "acm", StartTime: start, EndTime: start.Add(3 * time.Hour),
		CreatedBy: owner.ID, Status: model.ContestStatusPublished,
		IsRated: true, RatedForMin: 0, RatedForMax: 9999,
	}
	if err := contestStore.Create(ctx, contest); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	// The round's manager: this is the grant that makes them staff, and it is
	// deliberately separate from any problem-authoring right.
	if err := contestStore.AddPermission(ctx, contest.ID, roundManager.ID, "manager"); err != nil {
		t.Fatalf("grant manager: %v", err)
	}

	mkSub := func(status model.SubmissionStatus) string {
		id := uuid.New().String()
		if err := subStore.Create(ctx, &model.Submission{
			ID: id, ProblemID: problemID, UserID: solver.ID, ContestID: contest.ID,
			Language: "cpp-gpp-64", SourceCode: "int main(){}", Status: status,
			SubmissionType: model.SubmissionTypeCode,
		}); err != nil {
			t.Fatalf("create submission: %v", err)
		}
		return id
	}

	defer func() {
		db.ExecContext(ctx, "DELETE FROM submissions WHERE problem_id=$1", problemID)
		db.ExecContext(ctx, "DELETE FROM contest_permissions WHERE contest_id=$1", contest.ID)
		db.ExecContext(ctx, "DELETE FROM contests WHERE id=$1", contest.ID)
		db.ExecContext(ctx, "DELETE FROM problems WHERE id=$1", problemID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id IN ($1,$2,$3,$4)", owner.ID, roundManager.ID, stranger.ID, solver.ID)
	}()

	t.Run("a setter accountable for neither is refused", func(t *testing.T) {
		subID := mkSub(model.StatusWA)
		rec := serveSkipContest(t, h, contest.ID, subID, stranger, "looked suspicious")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("stranger skip = %d, want 403 (body=%q)", rec.Code, rec.Body.String())
		}
		// The refusal must be a refusal: the submission is untouched.
		got, _ := subStore.GetByID(ctx, subID)
		if got.Status != model.StatusWA {
			t.Fatalf("status = %q after a refused skip, want it unchanged", got.Status)
		}
	})

	t.Run("a missing reason is refused", func(t *testing.T) {
		subID := mkSub(model.StatusWA)
		rec := serveSkipContest(t, h, contest.ID, subID, owner, "   ")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("reasonless skip = %d, want 400 (body=%q)", rec.Code, rec.Body.String())
		}
		// A void nobody can explain later is worse than the submission it
		// removed, so the requirement is enforced, not merely documented.
		got, _ := subStore.GetByID(ctx, subID)
		if got.Status != model.StatusWA {
			t.Fatalf("status = %q after a reasonless skip, want it unchanged", got.Status)
		}
	})

	t.Run("the problem's author may void", func(t *testing.T) {
		subID := mkSub(model.StatusAC)
		rec := serveSkipContest(t, h, contest.ID, subID, owner, "resubmitted from a second account")
		if rec.Code != http.StatusOK {
			t.Fatalf("owner skip = %d, want 200 (body=%q)", rec.Code, rec.Body.String())
		}
		got, _ := subStore.GetByID(ctx, subID)
		if got.Status != model.StatusSkipped {
			t.Fatalf("status = %q, want %q", got.Status, model.StatusSkipped)
		}
	})

	t.Run("the round's manager may void", func(t *testing.T) {
		subID := mkSub(model.StatusTLE)
		rec := serveSkipContest(t, h, contest.ID, subID, roundManager, "copied from a public solution")
		if rec.Code != http.StatusOK {
			t.Fatalf("manager skip = %d, want 200 (body=%q)", rec.Code, rec.Body.String())
		}
		got, _ := subStore.GetByID(ctx, subID)
		if got.Status != model.StatusSkipped {
			t.Fatalf("status = %q, want %q", got.Status, model.StatusSkipped)
		}
	})

	t.Run("an admin may void", func(t *testing.T) {
		admin := newUser(model.RoleAdmin)
		defer db.ExecContext(ctx, "DELETE FROM users WHERE id=$1", admin.ID)
		subID := mkSub(model.StatusRE)
		rec := serveSkipContest(t, h, contest.ID, subID, admin, "ruled void by the jury")
		if rec.Code != http.StatusOK {
			t.Fatalf("admin skip = %d, want 200 (body=%q)", rec.Code, rec.Body.String())
		}
	})

	t.Run("already skipped is a conflict, not a second void", func(t *testing.T) {
		subID := mkSub(model.StatusWA)
		if rec := serveSkipContest(t, h, contest.ID, subID, owner, "first"); rec.Code != http.StatusOK {
			t.Fatalf("first skip = %d, want 200", rec.Code)
		}
		rec := serveSkipContest(t, h, contest.ID, subID, owner, "second")
		if rec.Code != http.StatusConflict {
			t.Fatalf("second skip = %d, want 409 (body=%q)", rec.Code, rec.Body.String())
		}
	})

	t.Run("a submission the judge still owns is refused", func(t *testing.T) {
		subID := mkSub(model.StatusJudging)
		rec := serveSkipContest(t, h, contest.ID, subID, owner, "too early")
		if rec.Code != http.StatusConflict {
			t.Fatalf("in-flight skip = %d, want 409 (body=%q)", rec.Code, rec.Body.String())
		}
	})

	t.Run("a submission from another round cannot be reached through this one", func(t *testing.T) {
		other := &model.Contest{
			ID: uuid.New().String(), Title: "Other " + uuid.New().String()[:8],
			Type: "acm", Format: "acm", StartTime: start, EndTime: start.Add(3 * time.Hour),
			CreatedBy: owner.ID, Status: model.ContestStatusPublished,
			IsRated: true, RatedForMin: 0, RatedForMax: 9999,
		}
		if err := contestStore.Create(ctx, other); err != nil {
			t.Fatalf("create other contest: %v", err)
		}
		defer db.ExecContext(ctx, "DELETE FROM contests WHERE id=$1", other.ID)

		subID := uuid.New().String()
		if err := subStore.Create(ctx, &model.Submission{
			ID: subID, ProblemID: problemID, UserID: solver.ID, ContestID: other.ID,
			Language: "cpp-gpp-64", SourceCode: "int main(){}", Status: model.StatusWA,
			SubmissionType: model.SubmissionTypeCode,
		}); err != nil {
			t.Fatalf("create submission: %v", err)
		}
		// The manager of `contest` must not reach a submission made in `other`,
		// even though the problem is the same one they can author.
		rec := serveSkipContest(t, h, contest.ID, subID, roundManager, "wrong round")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("cross-round skip = %d, want 404 (body=%q)", rec.Code, rec.Body.String())
		}
		got, _ := subStore.GetByID(ctx, subID)
		if got.Status != model.StatusWA {
			t.Fatalf("status = %q, want it unchanged", got.Status)
		}
	})
}

func serveSkipContest(t *testing.T, h *SubmissionHandler, contestID, subID string, u *model.User, reason string) *httptest.ResponseRecorder {
	t.Helper()
	body := bytes.NewBufferString(`{"reason":` + jsonString(reason) + `}`)
	req := httptest.NewRequest(http.MethodPost, "/api/contests/"+contestID+"/submissions/"+subID+"/skip", body)
	rctx := chi.NewRouteContext()
	// Both params, exactly as the route declares them: {id} is the CONTEST.
	rctx.URLParams.Add("id", contestID)
	rctx.URLParams.Add("submissionId", subID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	if u != nil {
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey,
			&auth.Claims{UserID: u.ID, Username: u.Username, Role: u.Role}))
	}
	rec := httptest.NewRecorder()
	h.SkipContestSubmission(rec, req)
	return rec
}

// jsonString encodes s so a reason containing a quote or a newline cannot make
// the request body invalid and turn a permission case into a 400.
func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}
