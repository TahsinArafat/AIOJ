package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * A private problem (visible = false) belongs to its authors. It used to be
 * reachable by any signed-in account through /api/problems/{slug}/export, which
 * shipped the statement and every test file for any slug, and anonymously
 * through /permissions, /submissions, /i18n, /editorials and /stats. The route
 * that leaked most was behind a feature gate that only requires a login, so
 * "setter sees another setter's draft" was never a role problem: there was no
 * check at all.
 *
 * These tests pin the shared answer — canReadProblem — at each entry point.
 */

func privateProblem() *model.Problem {
	return &model.Problem{
		ID:        "p1",
		Slug:      "draft",
		Title:     "Unpublished Draft",
		Visible:   false,
		CreatedBy: "author1",
	}
}

// asRequest attaches claims to a request the way AuthMiddleware would.
func asRequest(method, target string, claims *auth.Claims) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	if claims != nil {
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))
	}
	return req
}

// withURLParams sets the chi params a handler reads, since the router is not in
// the path here.
func withURLParams(req *http.Request, kv ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(kv); i += 2 {
		rctx.URLParams.Add(kv[i], kv[i+1])
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestCanReadProblem(t *testing.T) {
	draft := privateProblem()
	published := privateProblem()
	published.Visible = true

	cases := []struct {
		name   string
		p      *model.Problem
		claims *auth.Claims
		access map[string]string
		want   bool
	}{
		{"published problem is public", published, nil, nil, true},
		{"draft is closed to anonymous", draft, nil, nil, false},
		{"draft is closed to a plain user", draft, &auth.Claims{UserID: "u", Role: "user"}, nil, false},
		// The reported bug: role "setter" must grant nothing by itself.
		{"draft is closed to a different setter", draft, &auth.Claims{UserID: "setter2", Role: "setter"}, nil, false},
		{"draft is open to its creator", draft, &auth.Claims{UserID: "author1", Role: "setter"}, nil, true},
		{"draft is open to an admin", draft, &auth.Claims{UserID: "admin1", Role: "admin"}, nil, true},
		{"draft is open to a co-author", draft, &auth.Claims{UserID: "co1", Role: "setter"}, map[string]string{"co1/p1": "co_author"}, true},
		{"draft is open to a tester", draft, &auth.Claims{UserID: "t1", Role: "setter"}, map[string]string{"t1/p1": "tester"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &mockProblemStore{access: tc.access}
			if got := canReadProblem(context.Background(), store, tc.p, tc.claims); got != tc.want {
				t.Fatalf("canReadProblem = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExportRefusesAnotherSettersDraft(t *testing.T) {
	cases := []struct {
		name   string
		claims *auth.Claims
		access map[string]string
		want   int
	}{
		{"anonymous", nil, nil, http.StatusForbidden},
		{"unrelated setter", &auth.Claims{UserID: "setter2", Role: "setter"}, nil, http.StatusForbidden},
		{"plain user", &auth.Claims{UserID: "u", Role: "user"}, nil, http.StatusForbidden},
		{"tester may read but not export", &auth.Claims{UserID: "t1", Role: "setter"}, map[string]string{"t1/p1": "tester"}, http.StatusForbidden},
		{"creator", &auth.Claims{UserID: "author1", Role: "setter"}, nil, http.StatusOK},
		{"admin", &auth.Claims{UserID: "admin1", Role: "admin"}, nil, http.StatusOK},
		{"co-author", &auth.Claims{UserID: "co1", Role: "setter"}, map[string]string{"co1/p1": "co_author"}, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &mockProblemStore{getBySlug: privateProblem(), access: tc.access}
			h := NewProblemHandler(store)
			req := withURLParams(asRequest("GET", "/api/problems/draft/export", tc.claims), "slug", "draft")
			w := httptest.NewRecorder()

			h.Export(w, req)

			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
}

func TestListPermissionsRefusesAnotherSettersDraft(t *testing.T) {
	store := &mockProblemStore{getBySlug: privateProblem()}
	h := NewProblemHandler(store)

	req := withURLParams(asRequest("GET", "/api/problems/draft/permissions", &auth.Claims{UserID: "setter2", Role: "setter"}), "slug", "draft")
	w := httptest.NewRecorder()
	h.ListPermissions(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: a draft's collaborators are private", w.Code)
	}
}

func TestProblemStatsRefusesAnotherSettersDraft(t *testing.T) {
	store := &mockProblemStore{
		getByID: privateProblem(),
	}
	// GetProblemStats would consult the submission store; the guard must stop the
	// request before that happens, so a nil subStore is the assertion.
	h := NewStatsHandler(nil, store)

	req := withURLParams(asRequest("GET", "/api/stats/problems/p1", &auth.Claims{UserID: "setter2", Role: "setter"}), "problemId", "p1")
	w := httptest.NewRecorder()
	h.GetProblemStats(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for another setter's draft", w.Code)
	}
}
