package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

/**
 * Editorials were add-only: the store had no Update or Delete, and the routes
 * did not exist, so a typo in an editorial was permanent. These pin the two
 * endpoints and, more importantly, who may call them.
 *
 * Ownership of the row is what grants the right — an author fixing their own
 * editorial does not need a role on the problem. Anyone else is told the
 * editorial does not exist rather than that they may not touch it, so the
 * endpoint cannot be used to probe for ids.
 */

type editorialStub struct {
	byID    map[string]*model.Editorial
	updated []string
	deleted []string
}

func (s *editorialStub) Create(context.Context, *model.Editorial) error { return nil }

func (s *editorialStub) Update(_ context.Context, e *model.Editorial) error {
	s.updated = append(s.updated, e.ID)
	if row, ok := s.byID[e.ID]; ok {
		row.Title, row.Content, row.Approach = e.Title, e.Content, e.Approach
	}
	return nil
}

func (s *editorialStub) Delete(_ context.Context, id string) error {
	s.deleted = append(s.deleted, id)
	return nil
}

func (s *editorialStub) GetByID(_ context.Context, id string) (*model.Editorial, error) {
	return s.byID[id], nil
}

func (s *editorialStub) GetForManage(_ context.Context, id string) (*model.Editorial, error) {
	return s.byID[id], nil
}

func (s *editorialStub) GetByProblem(context.Context, string) ([]model.Editorial, error) {
	return nil, nil
}

func (s *editorialStub) GetOfficialSolution(context.Context, string) (*model.Editorial, error) {
	return nil, nil
}

func (s *editorialStub) List(context.Context, int, int) ([]model.Editorial, int, error) {
	return nil, 0, nil
}

func (s *editorialStub) UpsertOfficial(context.Context, *model.Editorial) error { return nil }

func (s *editorialStub) UpsertOfficialSolution(context.Context, *model.Editorial) error { return nil }

// manageProblemStore answers GetByID and HasAccess for the authorization checks
// these handlers run. `coAuthor` names the one user who holds a problem role.
// The embedded interface is nil: these handlers call only the two methods below,
// and a call to anything else panics rather than silently returning a zero value.
type manageProblemStore struct {
	store.ProblemStore
	problem  *model.Problem
	coAuthor string
}

func (s *manageProblemStore) GetByID(context.Context, string) (*model.Problem, error) {
	return s.problem, nil
}

func (s *manageProblemStore) HasAccess(_ context.Context, _, userID string, _ ...string) bool {
	return s.coAuthor != "" && userID == s.coAuthor
}

func manageHandler() (*EditorialHandler, *editorialStub) {
	es := &editorialStub{byID: map[string]*model.Editorial{
		"e1": {
			ID:        "e1",
			ProblemID: "p1",
			UserID:    "author1",
			Title:     "Original",
			Content:   "Original body",
		},
	}}
	h := NewEditorialHandler(es, &manageProblemStore{
		problem:  &model.Problem{ID: "p1", CreatedBy: "owner1"},
		coAuthor: "coauthor1",
	}, nil)
	return h, es
}

func manageRequest(method, body string, claims *auth.Claims) *http.Request {
	req := asRequest(method, "/api/editorials/e1", claims)
	req.Header.Set("Content-Type", "application/json")
	if body != "" {
		req.Body = io.NopCloser(strings.NewReader(body))
	}
	return withURLParams(req, "id", "e1")
}

func TestUpdateEditorialAllowsAuthor(t *testing.T) {
	h, es := manageHandler()
	req := manageRequest(http.MethodPut, `{"title":"Fixed","content":"Fixed body"}`, &auth.Claims{UserID: "author1"})
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("author update: got %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if len(es.updated) != 1 {
		t.Fatalf("store saw %d updates, want 1", len(es.updated))
	}
	if es.byID["e1"].Title != "Fixed" {
		t.Fatalf("title = %q, want Fixed", es.byID["e1"].Title)
	}
}

func TestUpdateEditorialAllowsProblemOwnerAndAdmin(t *testing.T) {
	for _, tc := range []struct {
		name   string
		claims *auth.Claims
	}{
		{"problem owner", &auth.Claims{UserID: "owner1"}},
		{"co-author", &auth.Claims{UserID: "coauthor1"}},
		{"admin", &auth.Claims{UserID: "someone-else", Role: "admin"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := manageHandler()
			req := manageRequest(http.MethodPut, `{"title":"Fixed","content":"Fixed body"}`, tc.claims)
			rec := httptest.NewRecorder()

			h.Update(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("%s: got %d, want 200 (body %s)", tc.name, rec.Code, rec.Body.String())
			}
		})
	}
}

// A stranger gets 404, not 403: a 403 would confirm the id exists.
func TestUpdateEditorialHidesExistenceFromStrangers(t *testing.T) {
	h, es := manageHandler()
	req := manageRequest(http.MethodPut, `{"title":"Hijacked","content":"x"}`, &auth.Claims{UserID: "stranger"})
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("stranger update: got %d, want 404", rec.Code)
	}
	if len(es.updated) != 0 {
		t.Fatalf("stranger reached the store: %v", es.updated)
	}
	if es.byID["e1"].Title != "Original" {
		t.Fatalf("row was modified by a stranger: %q", es.byID["e1"].Title)
	}
}

func TestUpdateEditorialRejectsEmptyTitleOrContent(t *testing.T) {
	h, _ := manageHandler()
	for _, body := range []string{`{"title":"","content":"body"}`, `{"title":"t","content":"  "}`} {
		req := manageRequest(http.MethodPut, body, &auth.Claims{UserID: "author1"})
		rec := httptest.NewRecorder()

		h.Update(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: got %d, want 400", body, rec.Code)
		}
	}
}

func TestDeleteEditorialAllowsAuthorAndRemovesRow(t *testing.T) {
	h, es := manageHandler()
	req := manageRequest(http.MethodDelete, "", &auth.Claims{UserID: "author1"})
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("author delete: got %d, want 204 (body %s)", rec.Code, rec.Body.String())
	}
	if len(es.deleted) != 1 || es.deleted[0] != "e1" {
		t.Fatalf("store deletions = %v, want [e1]", es.deleted)
	}
}

func TestDeleteEditorialHidesExistenceFromStrangers(t *testing.T) {
	h, es := manageHandler()
	req := manageRequest(http.MethodDelete, "", &auth.Claims{UserID: "stranger"})
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("stranger delete: got %d, want 404", rec.Code)
	}
	if len(es.deleted) != 0 {
		t.Fatalf("stranger reached the store: %v", es.deleted)
	}
}

func TestEditorialManagementRequiresAuth(t *testing.T) {
	h, _ := manageHandler()

	put := httptest.NewRecorder()
	h.Update(put, manageRequest(http.MethodPut, `{"title":"t","content":"c"}`, nil))
	if put.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated update: got %d, want 401", put.Code)
	}

	del := httptest.NewRecorder()
	h.Delete(del, manageRequest(http.MethodDelete, "", nil))
	if del.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated delete: got %d, want 401", del.Code)
	}
}

func TestUpdateEditorialIgnoresIsOfficialAndProblemID(t *testing.T) {
	h, es := manageHandler()
	es.byID["e1"].IsOfficial = false
	body, _ := json.Marshal(map[string]any{
		"title":       "Fixed",
		"content":     "Fixed body",
		"is_official": true,
		"problem_id":  "other-problem",
	})
	req := manageRequest(http.MethodPut, string(body), &auth.Claims{UserID: "author1"})
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	// Promotion controls who may read the editorial, so an edit must not grant it.
	if es.byID["e1"].IsOfficial {
		t.Fatal("edit promoted the editorial to official")
	}
	if es.byID["e1"].ProblemID != "p1" {
		t.Fatalf("edit moved the editorial to %q", es.byID["e1"].ProblemID)
	}
}

func TestUpdateEditorialUnknownIDIs404(t *testing.T) {
	h, _ := manageHandler()
	req := asRequest(http.MethodPut, "/api/editorials/missing", &auth.Claims{UserID: "author1"})
	req.Body = io.NopCloser(strings.NewReader(`{"title":"t","content":"c"}`))
	req = withURLParams(req, "id", "missing")
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown id: got %d, want 404", rec.Code)
	}
}
