package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
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
// caller does not own the problem.
func TestRejudgeProblemSubmissionsForbiddenForStranger(t *testing.T) {
	h := &SubmissionHandler{probStore: &mockProblemStore{
		getBySlug: &model.Problem{ID: "p1", Slug: "two-sum"},
		access:    map[string]string{"owner1/p1": "owner"},
	}}

	req := httptest.NewRequest("POST", "/api/problems/two-sum/submissions/rejudge", strings.NewReader(`{}`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, &auth.Claims{UserID: "stranger", Role: "user"}))
	req = withURLParam(req, "slug", "two-sum")
	rec := httptest.NewRecorder()

	h.RejudgeProblemSubmissions(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
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
