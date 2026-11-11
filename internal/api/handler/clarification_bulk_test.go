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

// A bulk answer is authorized for one contest, so the ids it names have to be
// checked against that contest. A judge of round A who guessed the ids of round
// B's questions would otherwise be able to answer them: the permission check is
// on the contest in the URL, not on the rows being written.
type bulkClarificationStore struct {
	rows  map[string]*model.Clarification
	asked []string
}

func (s *bulkClarificationStore) Create(context.Context, *model.Clarification) error { return nil }
func (s *bulkClarificationStore) Answer(context.Context, string, string, string, bool) error {
	return nil
}
func (s *bulkClarificationStore) AnswerMany(_ context.Context, ids []string, answer, by string, isPublic bool) error {
	s.asked = ids
	for _, id := range ids {
		s.rows[id].Answer = &answer
	}
	return nil
}
func (s *bulkClarificationStore) GetByID(_ context.Context, id string) (*model.Clarification, error) {
	return s.rows[id], nil
}
func (s *bulkClarificationStore) ListByContest(context.Context, string, *string, *string, *bool) ([]model.Clarification, error) {
	return nil, nil
}

type bulkContestStore struct {
	contests map[string]*model.Contest
	allowed  bool
}

func (s *bulkContestStore) GetByID(_ context.Context, id string) (*model.Contest, error) {
	return s.contests[id], nil
}
func (s *bulkContestStore) HasAccess(context.Context, string, string, ...string) bool {
	return s.allowed
}

func bulkHandler(t *testing.T, mine map[string]*model.Clarification, allowed bool) (*ClarificationHandler, *bulkClarificationStore) {
	t.Helper()
	cs := &bulkClarificationStore{rows: mine}
	cts := &bulkContestStore{
		contests: map[string]*model.Contest{"mine": {ID: "mine"}, "theirs": {ID: "theirs"}},
		allowed:  allowed,
	}
	return NewClarificationHandler(cs, cts), cs
}

// The caller is a judge of "mine" but asks the ids of "theirs" be answered. The
// request is refused outright: those rows are not this round's to answer.
func TestBulkAnswerRefusesIdsFromAnotherContest(t *testing.T) {
	mine := map[string]*model.Clarification{
		"theirs-1": {ID: "theirs-1", ContestID: "theirs", Question: "not mine"},
	}
	h, store := bulkHandler(t, mine, true)

	body, _ := json.Marshal(map[string]any{
		"ids":       []string{"theirs-1"},
		"answer":    "answered anyway",
		"is_public": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/contests/mine/clarifications/bulk-answer", strings.NewReader(string(body)))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey,
		&auth.Claims{UserID: "judge", Role: "user"}))
	// The handler reads the contest from the route, as it does in production.
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("contestId", "mine")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.AnswerMany(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for ids outside the contest", w.Code)
	}
	if len(store.asked) != 0 {
		t.Fatalf("nothing should have been written, but wrote to %v", store.asked)
	}
}

// A judge answers their own round's questions in bulk; only those rows change.
func TestBulkAnswerWritesOnlyTheAuthorizedRows(t *testing.T) {
	mine := map[string]*model.Clarification{
		"a-1": {ID: "a-1", ContestID: "mine", Question: "mine one"},
		"a-2": {ID: "a-2", ContestID: "mine", Question: "mine two"},
		"b-1": {ID: "b-1", ContestID: "theirs", Question: "theirs"},
	}
	h, store := bulkHandler(t, mine, true)

	body, _ := json.Marshal(map[string]any{
		"ids":       []string{"a-1", "a-2", "b-1"},
		"answer":    "please see the FAQ",
		"is_public": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/contests/mine/clarifications/bulk-answer", strings.NewReader(string(body)))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey,
		&auth.Claims{UserID: "judge", Role: "user"}))
	// The handler reads the contest from the route, as it does in production.
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("contestId", "mine")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.AnswerMany(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var res struct {
		Answered int `json:"answered"`
	}
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.Answered != 2 {
		t.Fatalf("answered = %d, want 2", res.Answered)
	}
	if len(store.asked) != 2 {
		t.Fatalf("wrote to %d rows, want 2", len(store.asked))
	}
	for _, id := range store.asked {
		if id == "b-1" {
			t.Fatalf("wrote to a row of another contest: %v", store.asked)
		}
	}
}
