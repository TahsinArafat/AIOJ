package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * Adding a problem collaborator answered 500 for every submission.
 *
 * The collaborator form on the problem workspace is labelled "Username" and its
 * client sends {"username": "...", "access_level": "co_author"}. The handler read
 * the field `user_id` instead, so it saw an empty string and handed it to
 *
 *     INSERT INTO problem_permissions(problem_id, user_id, ...)
 *
 * where user_id is a uuid column. Postgres rejected it with 22P02 "invalid input
 * syntax for type uuid" and the handler turned that into a bare 500. The bug was
 * invisible from the client: the request looked well-formed, the form said
 * Username, and there was nothing on screen to correct.
 *
 * Two smaller faults sat behind it. access_level reached a CHECK constraint
 * unvalidated, and model.AccessLevelCoAuthor was the string "co-author" while the
 * column accepts "co_author" — so the constant itself could not be stored.
 */

// recordingStore wraps the shared mock to record what AddPermission receives, so a
// test can assert on the id the handler resolved rather than only on the status.
type recordingStore struct {
	*mockProblemStore
	gotUserID string
	gotLevel  string
	written   bool
}

func (s *recordingStore) AddPermission(_ context.Context, _, userID, level string) error {
	s.gotUserID, s.gotLevel, s.written = userID, level, true
	return nil
}

// resolveStub answers the two lookups resolveUserRef makes.
type resolveStub struct {
	id       string
	username string
}

func (s *resolveStub) GetByID(_ context.Context, id string) (*model.User, error) {
	if id != s.id {
		return nil, nil
	}
	return &model.User{ID: s.id, Username: s.username}, nil
}

func (s *resolveStub) GetByUsername(_ context.Context, name string) (*model.User, error) {
	if name != s.username {
		return nil, nil
	}
	return &model.User{ID: s.id, Username: s.username}, nil
}

const collaboratorID = "0a0a0a0a-1111-4222-8333-444455556666"

// newCollaboratorHandler builds a handler whose caller owns the draft, so the
// authorization step does not mask the input handling under test.
func newCollaboratorHandler() (*ProblemHandler, *recordingStore) {
	store := &recordingStore{mockProblemStore: &mockProblemStore{
		getBySlug: privateProblem(),
		access:    map[string]string{"author1/p1": "owner"},
	}}
	h := NewProblemHandler(store).WithUsers(&resolveStub{id: collaboratorID, username: "newcollab"})
	return h, store
}

func postCollaborator(h *ProblemHandler, body string) *httptest.ResponseRecorder {
	claims := &auth.Claims{UserID: "author1", Role: "setter"}
	req := asRequest(http.MethodPost, "/api/problems/draft/permissions", claims)
	req = withURLParams(req, "slug", "draft")
	req.Body = io.NopCloser(strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.AddPermission(rec, req)
	return rec
}

// The reported bug: the form's own payload must be accepted, not 500.
func TestAddCollaboratorAcceptsAUsername(t *testing.T) {
	h, store := newCollaboratorHandler()

	rec := postCollaborator(h, `{"username":"newcollab","access_level":"co_author"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if store.gotUserID != collaboratorID {
		t.Fatalf("added %q, want the resolved account id %q", store.gotUserID, collaboratorID)
	}
	if store.gotLevel != model.AccessLevelCoAuthor {
		t.Fatalf("level = %q, want %q", store.gotLevel, model.AccessLevelCoAuthor)
	}
}

// An explicit id stays valid input, so callers that already know it do not regress.
func TestAddCollaboratorAcceptsAUserID(t *testing.T) {
	h, store := newCollaboratorHandler()

	rec := postCollaborator(h, `{"user_id":"`+collaboratorID+`","access_level":"tester"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if store.gotUserID != collaboratorID {
		t.Fatalf("added %q, want %q", store.gotUserID, collaboratorID)
	}
}

// An unknown account is the expected operator mistake, so it must be a 404 with a
// message rather than a 500 that reads as a server fault.
func TestAddCollaboratorRejectsAnUnknownUsername(t *testing.T) {
	h, store := newCollaboratorHandler()

	rec := postCollaborator(h, `{"username":"nobody","access_level":"co_author"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for an unknown user", rec.Code)
	}
	if store.written {
		t.Fatal("nothing must be written when the user does not exist")
	}
}

// The empty-reference case is the exact shape of the original failure: if it ever
// reaches the store, the uuid column fails and we are back to the reported 500.
func TestAddCollaboratorNeverWritesAnEmptyUserID(t *testing.T) {
	h, store := newCollaboratorHandler()

	rec := postCollaborator(h, `{"access_level":"co_author"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when no user is named", rec.Code)
	}
	if store.written {
		t.Fatalf("an empty reference reached the store as %q", store.gotUserID)
	}
}

func TestAddCollaboratorRejectsAnUnknownLevel(t *testing.T) {
	h, store := newCollaboratorHandler()

	rec := postCollaborator(h, `{"username":"newcollab","access_level":"superuser"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an unknown level", rec.Code)
	}
	if store.written {
		t.Fatal("nothing must be written for an invalid level")
	}
}

// The constant must match the column's CHECK constraint. It said "co-author" while
// problem_permissions accepts "co_author", so anything that used the constant
// rather than a literal produced a value the database rejects.
func TestCoAuthorConstantMatchesTheColumn(t *testing.T) {
	if model.AccessLevelCoAuthor != "co_author" {
		t.Fatalf("AccessLevelCoAuthor = %q, want co_author to match the CHECK constraint",
			model.AccessLevelCoAuthor)
	}
}
