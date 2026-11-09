package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

// contestStub is the minimum contest store the problem handler needs.
type contestStub struct {
	contest   *model.Contest
	problem   *model.Problem
	judge     bool
	player    bool
	completed bool
}

func (s *contestStub) GetByID(_ context.Context, _ string) (*model.Contest, error) {
	return s.contest, nil
}
func (s *contestStub) HasAccess(_ context.Context, _, _ string, _ ...string) bool { return s.judge }
func (s *contestStub) GetContestProblemByIndex(_ context.Context, _, _ string) (*model.Problem, error) {
	return s.problem, nil
}
func (s *contestStub) IsParticipant(_ context.Context, _, _ string) (bool, error) {
	return s.player, nil
}

func contestProblemAs(t *testing.T, stub *contestStub, store *mockProblemStore, claims *auth.Claims) map[string]interface{} {
	t.Helper()
	h := NewContestProblemHandler(stub, store)
	req := httptest.NewRequest("GET", "/api/contests/c1/problems/A", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{Keys: []string{"contestId", "index"}, Values: []string{"c1", "A"}},
	}))
	if claims != nil {
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))
	}
	w := httptest.NewRecorder()
	h.GetByIndex(w, req)

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode (%d): %v %s", w.Code, err, w.Body.String())
	}
	return body
}

func solvedContest() *model.Contest {
	now := time.Now()
	return &model.Contest{ID: "c1", Visible: true, StartTime: now.Add(-time.Hour), EndTime: now.Add(time.Hour)}
}

func TestContestProblemHidesSpoilersWhileRunning(t *testing.T) {
	stub := &contestStub{contest: solvedContest(), problem: spoiledProblem()}
	store := &mockProblemStore{unreleasedContest: true}

	body := contestProblemAs(t, stub, store, nil)
	problem := body["problem"].(map[string]interface{})

	// A missing key is as good as an empty one: the point is the value never ships.
	if difficulty, _ := problem["difficulty"].(string); difficulty != "" {
		t.Errorf("difficulty leaked: %q", difficulty)
	}
	if rating, _ := problem["rating"].(float64); rating != 0 {
		t.Errorf("rating leaked: %v", rating)
	}
	if tags, ok := problem["tags"].([]interface{}); ok && len(tags) > 0 {
		t.Errorf("tags leaked: %v", tags)
	}
	if hint, _ := problem["hint"].(string); hint != "" {
		t.Errorf("hint leaked: %q", hint)
	}
	if body["spoilers_hidden"] != true {
		t.Errorf("client was not told the spoilers are hidden: %v", body["spoilers_hidden"])
	}
	if problem["description"] != "body" {
		t.Errorf("the statement must stay readable, got %v", problem["description"])
	}
}

func TestContestProblemRevealsAfterTheContest(t *testing.T) {
	now := time.Now()
	ended := &model.Contest{ID: "c1", Visible: true, StartTime: now.Add(-2 * time.Hour), EndTime: now.Add(-time.Hour)}
	stub := &contestStub{contest: ended, problem: spoiledProblem(), player: true}
	store := &mockProblemStore{unreleasedContest: false}

	body := contestProblemAs(t, stub, store, &auth.Claims{UserID: "u1", Role: "user"})
	problem := body["problem"].(map[string]interface{})

	if problem["difficulty"] != "hard" {
		t.Errorf("difficulty should be visible after the contest, got %v", problem["difficulty"])
	}
	if body["spoilers_hidden"] != false {
		t.Errorf("spoilers_hidden = %v, want false", body["spoilers_hidden"])
	}
}

func TestContestProblemKeepsSpoilersForJudges(t *testing.T) {
	stub := &contestStub{contest: solvedContest(), problem: spoiledProblem(), judge: true}
	store := &mockProblemStore{unreleasedContest: true}

	body := contestProblemAs(t, stub, store, &auth.Claims{UserID: "judge1", Role: "user"})

	if body["problem"].(map[string]interface{})["difficulty"] != "hard" {
		t.Errorf("a judge must keep the full view: %v", body["problem"])
	}
}

func TestContestProblemKeepsSpoilersForTheAuthor(t *testing.T) {
	stub := &contestStub{contest: solvedContest(), problem: spoiledProblem()}
	store := &mockProblemStore{
		unreleasedContest: true,
		access:            map[string]string{"setter1/p1": "co_author"},
	}

	body := contestProblemAs(t, stub, store, &auth.Claims{UserID: "setter1", Role: "setter"})

	if body["problem"].(map[string]interface{})["difficulty"] != "hard" {
		t.Errorf("the author must keep the full view: %v", body["problem"])
	}
}
