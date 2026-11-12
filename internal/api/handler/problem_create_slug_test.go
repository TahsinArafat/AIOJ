package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lib/pq"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * The setter's create screen is a name prompt: it posts a title and nothing
 * else, and the server has to produce a problem from that alone. The slug used
 * to be the client's job, so a title-only request was rejected outright; these
 * tests pin the replacement — derive the slug, accept the empty description,
 * and disambiguate a name that is already taken.
 */

// slugStore records every problem handed to Create and can fail the first N
// inserts with a duplicate-slug error, so the retry path is exercised without a
// database.
type slugStore struct {
	mockProblemStore
	attempts []string
	dupes    int
}

func (s *slugStore) Create(_ context.Context, p *model.Problem) error {
	s.attempts = append(s.attempts, p.Slug)
	if len(s.attempts) <= s.dupes {
		return &pq.Error{Code: "23505"}
	}
	return nil
}

func createJSON(t *testing.T, h http.HandlerFunc, body string, claims *auth.Claims) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/problems", strings.NewReader(body))
	if claims != nil {
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))
	}
	w := httptest.NewRecorder()
	h(w, req)
	return w
}

func TestSlugFromTitle(t *testing.T) {
	cases := []struct {
		title string
		want  string
	}{
		{"Range Sum Queries", "range-sum-queries"},
		{"  Two Sum  ", "two-sum"},
		{"A+B Problem", "a-b-problem"},
		{"FizzBuzz!!", "fizzbuzz"},
		{"Segment Tree, Lazy Propagation (Hard)", "segment-tree-lazy-propagation-hard"},
	}
	for _, tc := range cases {
		if got := slugFromTitle(tc.title); got != tc.want {
			t.Errorf("slugFromTitle(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
}

func TestSlugFromTitleFallsBackWhenNothingIsLatin(t *testing.T) {
	// A CJK or emoji title has no ASCII left to build a URL from. It must still
	// produce a usable, non-empty segment — an empty slug would fail the UNIQUE
	// NOT NULL column and surface as an undiagnosable 500.
	for _, title := range []string{"素数筛法", "🎉", "···"} {
		got := slugFromTitle(title)
		if !strings.HasPrefix(got, "problem-") || len(got) <= len("problem-") {
			t.Errorf("slugFromTitle(%q) = %q, want a problem-<suffix> fallback", title, got)
		}
	}
}

func TestCreateAcceptsTitleOnly(t *testing.T) {
	store := &slugStore{}
	h := NewProblemHandler(store)

	w := createJSON(t, h.Create, `{"title":"Range Sum Queries"}`, &auth.Claims{UserID: "u1", Role: "setter"})

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", w.Code, w.Body.String())
	}
	if len(store.attempts) != 1 || store.attempts[0] != "range-sum-queries" {
		t.Fatalf("created slugs = %v, want [range-sum-queries]", store.attempts)
	}
	if !strings.Contains(w.Body.String(), `"description":""`) {
		t.Fatalf("body = %s, want an empty description rather than a rejected request", w.Body.String())
	}
}

func TestCreateRejectsMissingTitle(t *testing.T) {
	store := &slugStore{}
	h := NewProblemHandler(store)

	w := createJSON(t, h.Create, `{"description":"no name"}`, &auth.Claims{UserID: "u1", Role: "setter"})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: a problem with no name has nothing to derive a slug from", w.Code)
	}
	if len(store.attempts) != 0 {
		t.Fatalf("created slugs = %v, want none", store.attempts)
	}
}

func TestCreateDisambiguatesADerivedSlugCollision(t *testing.T) {
	// Two setters naming a problem "Two Sum" is ordinary. The first insert
	// collides, and the retry has to come back with a distinct slug instead of
	// handing the form a 409 it cannot act on.
	store := &slugStore{dupes: 2}
	h := NewProblemHandler(store)

	w := createJSON(t, h.Create, `{"title":"Two Sum"}`, &auth.Claims{UserID: "u1", Role: "setter"})

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", w.Code, w.Body.String())
	}
	want := []string{"two-sum", "two-sum-2", "two-sum-3"}
	if len(store.attempts) != len(want) {
		t.Fatalf("attempts = %v, want %v", store.attempts, want)
	}
	for i, slug := range want {
		if store.attempts[i] != slug {
			t.Fatalf("attempts = %v, want %v", store.attempts, want)
		}
	}
}

func TestCreateKeepsConflictForAnExplicitSlug(t *testing.T) {
	// An explicit slug is the caller's to resolve. The seeder relies on this:
	// it treats 409 as "already exists, carry on" on every re-run.
	store := &slugStore{dupes: 1}
	h := NewProblemHandler(store)

	w := createJSON(t, h.Create, `{"slug":"hello-aioj","title":"Hello, AIOJ!","description":"Print hi."}`,
		&auth.Claims{UserID: "u1", Role: "setter"})

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 for an explicit slug that exists", w.Code)
	}
	if len(store.attempts) != 1 {
		t.Fatalf("attempts = %v, want a single insert with no rewrite", store.attempts)
	}
}
