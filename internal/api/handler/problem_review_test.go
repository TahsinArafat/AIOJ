package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * Publishing is an admin decision. Before this, any setter could put a problem
 * in the public archive on create, on edit, or through any of the five
 * importers, and could announce a rated contest, with no review at all.
 *
 * These tests pin the three doors shut: create, edit, and the review endpoints
 * that are the only way to publish.
 */

// reviewStore records the review state a handler asked for, so the tests can
// assert on the decision rather than on a fake's canned response.
type reviewStore struct {
	mockProblemStore
	gotStatus string
	gotNote   string
	gotBy     string
}

func (r *reviewStore) SetReviewState(_ context.Context, _, status, note, reviewer string) error {
	r.gotStatus, r.gotNote, r.gotBy = status, note, reviewer
	return nil
}

func postReviewJSON(t *testing.T, h http.HandlerFunc, target, body string, claims *auth.Claims, kv ...string) *httptest.ResponseRecorder {
	t.Helper()
	keys := make([]string, 0, len(kv)/2)
	values := make([]string, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		keys = append(keys, kv[i])
		values = append(values, kv[i+1])
	}
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{Keys: keys, Values: values},
	}))
	if claims != nil {
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))
	}
	w := httptest.NewRecorder()
	h(w, req)
	return w
}

// The author proposes; the state moves to pending and nothing becomes public.
func TestSubmitForReview(t *testing.T) {
	cases := []struct {
		name   string
		claims *auth.Claims
		access map[string]string
		want   int
	}{
		{"anonymous", nil, nil, http.StatusForbidden},
		{"unrelated setter", &auth.Claims{UserID: "s2", Role: "setter"}, nil, http.StatusForbidden},
		{"the author", &auth.Claims{UserID: "author1", Role: "setter"}, nil, http.StatusOK},
		{"a co-author", &auth.Claims{UserID: "co1", Role: "setter"}, map[string]string{"co1/p1": "co_author"}, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &reviewStore{mockProblemStore: mockProblemStore{getBySlug: privateProblem(), access: tc.access}}
			h := NewProblemHandler(store)

			w := postReviewJSON(t, h.SubmitForReview, "/api/problems/draft/submit", "", tc.claims, "slug", "draft")

			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
			if tc.want == http.StatusOK && store.gotStatus != model.ProblemStatusPending {
				t.Fatalf("review state = %q, want pending", store.gotStatus)
			}
		})
	}
}

// A tester may read a draft but has no say over whether it is published.
func TestSubmitForReviewRejectsATester(t *testing.T) {
	store := &reviewStore{mockProblemStore: mockProblemStore{
		getBySlug: privateProblem(),
		access:    map[string]string{"t1/p1": "tester"},
	}}
	h := NewProblemHandler(store)

	w := postReviewJSON(t, h.SubmitForReview, "/api/problems/draft/submit", "",
		&auth.Claims{UserID: "t1", Role: "setter"}, "slug", "draft")

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: a tester cannot submit for review", w.Code)
	}
}

// ReviewProblem is the only path that publishes, and only an admin may call it.
func TestReviewProblemIsAdminOnly(t *testing.T) {
	cases := []struct {
		name   string
		claims *auth.Claims
		want   int
	}{
		{"anonymous", nil, http.StatusForbidden},
		{"setter", &auth.Claims{UserID: "s1", Role: "setter"}, http.StatusForbidden},
		// Even the problem's own author cannot approve it.
		{"the author", &auth.Claims{UserID: "author1", Role: "setter"}, http.StatusForbidden},
		{"admin", &auth.Claims{UserID: "a1", Role: "admin"}, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &reviewStore{mockProblemStore: mockProblemStore{getBySlug: privateProblem()}}
			h := NewProblemHandler(store)

			w := postReviewJSON(t, h.ReviewProblem, "/api/admin/problem-review/draft",
				`{"status":"published","note":"looks good"}`, tc.claims, "slug", "draft")

			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
			if tc.want == http.StatusOK {
				if store.gotStatus != model.ProblemStatusPublished {
					t.Fatalf("review state = %q, want published", store.gotStatus)
				}
				if store.gotBy != "a1" {
					t.Fatalf("reviewer = %q, want the admin's id", store.gotBy)
				}
			}
		})
	}
}

func TestReviewProblemRejectsAnUnknownStatus(t *testing.T) {
	store := &reviewStore{mockProblemStore: mockProblemStore{getBySlug: privateProblem()}}
	h := NewProblemHandler(store)

	w := postReviewJSON(t, h.ReviewProblem, "/api/admin/problem-review/draft",
		`{"status":"banana"}`, &auth.Claims{UserID: "a1", Role: "admin"}, "slug", "draft")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an unknown review state", w.Code)
	}
}

// Editing must not be a way around the queue: a non-admin's visible flag is
// ignored, and only an admin's edit can change the published state.
func TestUpdateCannotPublishForANonAdmin(t *testing.T) {
	draft := privateProblem()
	store := &mockProblemStore{getBySlug: draft, access: map[string]string{"author1/p1": "owner"}}
	h := NewProblemHandler(store)

	body := `{"title":"Unpublished Draft","description":"d","time_limit":1000,"memory_limit":262144,"difficulty":"easy","visible":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/problems/draft", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{Keys: []string{"slug"}, Values: []string{"draft"}},
	}))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey,
		&auth.Claims{UserID: "author1", Role: "setter"}))
	w := httptest.NewRecorder()

	h.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if store.updated == nil {
		t.Fatal("the update did not reach the store")
	}
	if store.updated.Status == model.ProblemStatusPublished {
		t.Fatal("a setter published a problem by editing it with visible:true")
	}
}

func TestReviewQueueDefaultsToPending(t *testing.T) {
	var asked string
	store := &reviewStore{}
	store.listStatus = func(status string) { asked = status }
	h := NewProblemHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/problem-review", nil)
	w := httptest.NewRecorder()
	h.ReviewQueue(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if asked != model.ProblemStatusPending {
		t.Fatalf("queue listed %q, want pending", asked)
	}
	var body struct {
		Data []model.ProblemListItem `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}
