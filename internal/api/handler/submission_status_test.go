package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

// withURLParam attaches a chi URL parameter, which chi.URLParam reads back.
func withURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func requestAs(claims *auth.Claims) *http.Request {
	r := httptest.NewRequest("GET", "/api/submissions/all", nil)
	if claims != nil {
		r = r.WithContext(context.WithValue(r.Context(), middleware.UserContextKey, claims))
	}
	return r
}

// The status page has to tell the viewer, per row, whether they can open the
// submission at all — a link to a row the API will 403 is worse than plain
// text.
func TestDecorateSetsViewerCapabilities(t *testing.T) {
	h := &SubmissionHandler{probStore: &mockProblemStore{access: map[string]string{
		"me/owned":    "owner",
		"me/coauthed": "co_author",
		"me/tested":   "tester",
	}}}
	subs := []model.Submission{
		{ID: "s1", ProblemID: "owned", UserID: "someone-else"},
		{ID: "s2", ProblemID: "coauthed", UserID: "someone-else"},
		{ID: "s3", ProblemID: "tested", UserID: "someone-else"},
		{ID: "s4", ProblemID: "untouched", UserID: "someone-else"},
		{ID: "s5", ProblemID: "untouched", UserID: "me"},
	}

	t.Run("setter manages owned and co-authored problems only", func(t *testing.T) {
		rows := append([]model.Submission(nil), subs...)
		h.decorate(requestAs(&auth.Claims{UserID: "me", Role: "setter"}), rows, &auth.Claims{UserID: "me", Role: "setter"})

		want := []struct{ view, rejudge bool }{
			{true, true},   // owner
			{true, true},   // co_author
			{false, false}, // tester: no code access, no rejudge
			{false, false}, // unrelated problem, someone else's row
			{true, false},  // own row on an unrelated problem
		}
		for i, w := range want {
			if rows[i].CanView != w.view || rows[i].CanRejudge != w.rejudge {
				t.Errorf("row %d (%s): got view=%v rejudge=%v, want view=%v rejudge=%v",
					i, rows[i].ID, rows[i].CanView, rows[i].CanRejudge, w.view, w.rejudge)
			}
		}
	})

	t.Run("admin manages everything", func(t *testing.T) {
		rows := append([]model.Submission(nil), subs...)
		claims := &auth.Claims{UserID: "admin1", Role: "admin"}
		h.decorate(requestAs(claims), rows, claims)
		for _, row := range rows {
			if !row.CanView || !row.CanRejudge {
				t.Errorf("row %s: admin got view=%v rejudge=%v, want both true", row.ID, row.CanView, row.CanRejudge)
			}
		}
	})

	t.Run("anonymous viewer sees no rows as openable", func(t *testing.T) {
		rows := append([]model.Submission(nil), subs...)
		h.decorate(requestAs(nil), rows, nil)
		for _, row := range rows {
			if row.CanView || row.CanRejudge {
				t.Errorf("row %s: anonymous got view=%v rejudge=%v, want both false", row.ID, row.CanView, row.CanRejudge)
			}
		}
	})
}

func TestCanManageProblem(t *testing.T) {
	h := &SubmissionHandler{probStore: &mockProblemStore{access: map[string]string{
		"u/p1": "owner",
		"u/p2": "co_author",
		"u/p3": "tester",
	}}}

	cases := []struct {
		name   string
		claims *auth.Claims
		prob   string
		want   bool
	}{
		{"admin always", &auth.Claims{UserID: "a", Role: "admin"}, "p3", true},
		{"owner", &auth.Claims{UserID: "u", Role: "setter"}, "p1", true},
		{"co-author", &auth.Claims{UserID: "u", Role: "setter"}, "p2", true},
		{"tester cannot rejudge", &auth.Claims{UserID: "u", Role: "setter"}, "p3", false},
		{"stranger", &auth.Claims{UserID: "u", Role: "user"}, "p3", false},
		{"anonymous", nil, "p1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := h.canManageProblem(requestAs(tc.claims), tc.prob); got != tc.want {
				t.Fatalf("canManageProblem(%s) = %v, want %v", tc.prob, got, tc.want)
			}
		})
	}
}

func TestPageParams(t *testing.T) {
	cases := []struct {
		query         string
		offset, limit int
	}{
		{"", 0, 50},
		{"?offset=100&limit=20", 100, 20},
		{"?offset=-5&limit=0", 0, 50},
		{"?limit=5000", 0, 50}, // clamped: no unbounded page sizes
	}
	for _, tc := range cases {
		r := httptest.NewRequest("GET", "/api/submissions/all"+tc.query, nil)
		offset, limit := pageParams(r)
		if offset != tc.offset || limit != tc.limit {
			t.Errorf("pageParams(%q) = (%d, %d), want (%d, %d)", tc.query, offset, limit, tc.offset, tc.limit)
		}
	}
}

// A rejudge request must be rejected before it reaches the queue when the
// caller answers for neither the submission nor the scope it names.
func TestRejudgeForbiddenForStranger(t *testing.T) {
	db := openRosterTestDB(t)
	defer db.Close()

	ctx := context.Background()
	users := postgres.NewUserStore(db)
	problems := postgres.NewProblemStore(db)
	contests := postgres.NewContestStore(db)
	subs := postgres.NewSubmissionStore(db)
	h := NewSubmissionHandler(subs, problems, contests, &nopQueue{}, nil, nil, "", nil, users)

	newUser := func(role string) *model.User {
		u := &model.User{
			ID: uuid.NewString(), Username: "rejudge_" + uuid.NewString()[:8],
			Email: uuid.NewString() + "@example.com", PasswordHash: "hashed", Role: role,
		}
		if err := users.Create(ctx, u); err != nil {
			t.Fatalf("create user %s: %v", role, err)
		}
		return u
	}

	owner := newUser(model.RoleSetter)
	stranger := newUser(model.RoleUser)
	solver := newUser(model.RoleUser)

	problemID := uuid.NewString()
	if err := problems.Create(ctx, &model.Problem{
		ID: problemID, Slug: "rejudge-" + uuid.NewString()[:8], Title: "Rejudge",
		Description: "d", TimeLimit: 1000, MemoryLimit: 262144, Difficulty: "easy",
		Tags: []string{}, CreatedBy: owner.ID,
		Status: model.ProblemStatusPublished, Visible: true,
	}); err != nil {
		t.Fatalf("create problem: %v", err)
	}
	subID := uuid.NewString()
	if err := subs.Create(ctx, &model.Submission{
		ID: subID, ProblemID: problemID, UserID: solver.ID,
		Language: "cpp-gpp-64", SourceCode: "int main(){}", Status: model.StatusWA,
		SubmissionType: model.SubmissionTypeCode,
	}); err != nil {
		t.Fatalf("create submission: %v", err)
	}
	defer func() {
		db.ExecContext(ctx, "DELETE FROM submissions WHERE problem_id=$1", problemID)
		db.ExecContext(ctx, "DELETE FROM problems WHERE id=$1", problemID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id IN ($1,$2,$3)", owner.ID, stranger.ID, solver.ID)
	}()

	cases := []struct {
		name   string
		caller *model.User
		body   string
	}{
		{"one submission, a stranger", stranger, `{"submission_id":"` + subID + `"}`},
		{"a problem, a stranger", stranger, `{"problem_id":"` + problemID + `"}`},
		{"nothing named at all", owner, `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claims := &auth.Claims{UserID: tc.caller.ID, Username: tc.caller.Username, Role: tc.caller.Role}
			req := httptest.NewRequest("POST", "/api/submissions/rejudge", strings.NewReader(tc.body))
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))
			rec := httptest.NewRecorder()

			h.Rejudge(rec, req)

			if rec.Code != http.StatusForbidden && rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 403 or 400, got %d body=%s", rec.Code, rec.Body.String())
			}
			// Whichever refusal it is, it has to be a refusal: the row keeps the
			// verdict it had and no work was handed to the judge.
			got, _ := subs.GetByID(ctx, subID)
			if got.Status != model.StatusWA {
				t.Fatalf("status = %q after a refused rejudge, want it unchanged", got.Status)
			}
		})
	}
}

// The two halves of the accountability rule are the same halves for both
// actions, so the per-row capability flags a listing carries have to agree with
// them: a round's judge may void and rejudge inside the round and nowhere else.
func TestDecorateCapabilitiesFollowAccountability(t *testing.T) {
	ownerStore := &mockProblemStore{access: map[string]string{"owner1/p1": "owner"}}
	h := &SubmissionHandler{probStore: ownerStore, contestStore: stubContestStore{isJudge: true}}

	subs := []model.Submission{
		{ID: "s1", ProblemID: "p1", UserID: "solver1"},
		{ID: "s2", ProblemID: "p2", UserID: "solver1", ContestID: "c1"},
		{ID: "s3", ProblemID: "p2", UserID: "solver1"},
	}
	req := requestAs(&auth.Claims{UserID: "owner1", Role: "setter"})
	h.decorate(req, subs, middleware.GetUserClaims(req))

	// Owns the problem.
	if !subs[0].CanRejudge || !subs[0].CanSkip {
		t.Error("the problem's owner must be offered both actions on its rows")
	}
	// Does not own the problem, but judges the round the row was made in.
	if !subs[1].CanRejudge || !subs[1].CanSkip {
		t.Error("a round's judge must be offered both actions on the round's rows")
	}
	// Neither: a stranger's practice row.
	if subs[2].CanRejudge || subs[2].CanSkip {
		t.Error("nobody may act on a row they do not answer for")
	}
	if subs[2].CanView {
		t.Error("and a row they did not submit is not theirs to open either")
	}

	// An onsite account is a competitor, never an organizer, whatever it owns.
	contestant := []model.Submission{{ID: "s4", ProblemID: "p1", UserID: "onsie1"}}
	req = requestAs(&auth.Claims{UserID: "onsie1", Role: model.RoleContestant})
	h.decorate(req, contestant, middleware.GetUserClaims(req))
	if contestant[0].CanRejudge || contestant[0].CanSkip {
		t.Error("a generated onsie account must not be offered organizer actions")
	}
}

// stubContestStore answers the one method the judge-internals check needs.
// Embedding the interface keeps the stub honest about the rest of the contract
// without reimplementing every method the check never calls.
type stubContestStore struct {
	store.ContestStore
	isJudge bool
}

func (s stubContestStore) HasAccess(_ context.Context, _, _ string, _ ...string) bool {
	return s.isJudge
}

// The judge's failure text is an internal detail: the contestant who got the
// system error must not receive it, the people who can act on it must.
func TestCanSeeJudgeInternals(t *testing.T) {
	sub := &model.Submission{ProblemID: "p1", UserID: "solver1", ContestID: "c1"}
	ownerStore := &mockProblemStore{access: map[string]string{"setter1/p1": "owner"}}

	cases := []struct {
		name   string
		claims *auth.Claims
		judge  bool
		want   bool
	}{
		{name: "admin", claims: &auth.Claims{UserID: "admin1", Role: "admin"}, want: true},
		{name: "problem owner", claims: &auth.Claims{UserID: "setter1", Role: "setter"}, want: true},
		{name: "contest judge", claims: &auth.Claims{UserID: "judge1", Role: "setter"}, judge: true, want: true},
		{name: "the submitter", claims: &auth.Claims{UserID: "solver1", Role: "user"}, want: false},
		{name: "anonymous", claims: nil, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &SubmissionHandler{probStore: ownerStore, contestStore: stubContestStore{isJudge: tc.judge}}
			if got := h.canSeeJudgeInternals(requestAs(tc.claims), sub); got != tc.want {
				t.Fatalf("canSeeJudgeInternals() = %v, want %v", got, tc.want)
			}
		})
	}
}
