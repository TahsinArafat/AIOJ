package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * Contest problems here are ordinary archive problems, so a problem that is live
 * in a contest is reachable from both the contest endpoint and the public one.
 * Difficulty, rating, tags and the hint give the intended solution away, so both
 * have to withhold them until every contest using the problem has finished —
 * hiding them in the UI alone is bypassable with curl.
 */

func spoiledProblem() *model.Problem {
	return &model.Problem{
		ID:              "p1",
		Slug:            "secret",
		Title:           "Secret",
		Description:     "body",
		Constraints:     "$1 \\le n \\le 10$",
		Hint:            "use a segment tree",
		Difficulty:      "hard",
		Rating:          2400,
		Tags:            []string{"dp", "segment-tree"},
		Visible:         true,
		SubmissionCount: 42,
		AcceptedCount:   7,
	}
}

func getBySlugWith(t *testing.T, store *mockProblemStore, claims *auth.Claims) *model.Problem {
	t.Helper()
	h := NewProblemHandler(store)
	req := httptest.NewRequest("GET", "/api/problems/secret", nil)
	if claims != nil {
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))
	}
	w := httptest.NewRecorder()
	h.GetBySlug(w, req)

	var got model.Problem
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	return &got
}

func TestGetBySlugHidesSpoilersWhileTheContestRuns(t *testing.T) {
	store := &mockProblemStore{getBySlug: spoiledProblem(), unreleasedContest: true}

	got := getBySlugWith(t, store, nil)

	if got.Difficulty != "" || got.Rating != 0 || len(got.Tags) != 0 || got.Hint != "" {
		t.Fatalf("spoilers leaked: difficulty=%q rating=%d tags=%v hint=%q",
			got.Difficulty, got.Rating, got.Tags, got.Hint)
	}
	if got.SubmissionCount != 0 || got.AcceptedCount != 0 {
		t.Fatalf("solve counts leaked: %d/%d", got.AcceptedCount, got.SubmissionCount)
	}
	// The statement itself must still be readable.
	if got.Description != "body" || got.Constraints == "" {
		t.Fatalf("statement was withheld along with the spoilers")
	}
}

func TestGetBySlugRevealsSpoilersOnceTheContestEnds(t *testing.T) {
	store := &mockProblemStore{getBySlug: spoiledProblem(), unreleasedContest: false}

	got := getBySlugWith(t, store, nil)

	if got.Difficulty != "hard" || got.Rating != 2400 || got.Hint == "" {
		t.Fatalf("spoilers should be visible after the contest: %+v", got)
	}
}

func TestGetBySlugKeepsSpoilersForTheAuthor(t *testing.T) {
	store := &mockProblemStore{
		getBySlug:         spoiledProblem(),
		unreleasedContest: true,
		access:            map[string]string{"setter1/p1": "owner"},
	}

	got := getBySlugWith(t, store, &auth.Claims{UserID: "setter1", Role: "setter"})

	if got.Difficulty != "hard" || got.Rating != 2400 || len(got.Tags) == 0 {
		t.Fatalf("the author must keep the full view: %+v", got)
	}
}

func TestGetBySlugKeepsSpoilersForAdmins(t *testing.T) {
	store := &mockProblemStore{getBySlug: spoiledProblem(), unreleasedContest: true}

	got := getBySlugWith(t, store, &auth.Claims{UserID: "admin1", Role: "admin"})

	if got.Difficulty != "hard" || got.Rating != 2400 {
		t.Fatalf("an admin must keep the full view: %+v", got)
	}
}

func TestListHidesSpoilersOnLiveContestProblems(t *testing.T) {
	store := &mockProblemStore{listItems: []model.ProblemListItem{
		{Slug: "live", Difficulty: "hard", Rating: 2400, InUnreleasedContest: true},
		{Slug: "archived", Difficulty: "easy", Rating: 800, InUnreleasedContest: false},
	}}

	items := listAs(t, store)

	if items[0].Difficulty != "" || items[0].Rating != 0 {
		t.Fatalf("live contest problem leaked its difficulty: %+v", items[0])
	}
	if items[1].Difficulty != "easy" || items[1].Rating != 800 {
		t.Fatalf("archived problem should keep its difficulty: %+v", items[1])
	}
}

func listAs(t *testing.T, store *mockProblemStore) []model.ProblemListItem {
	t.Helper()
	h := NewProblemHandler(store)
	req := httptest.NewRequest("GET", "/api/problems", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	var body struct {
		Data []model.ProblemListItem `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	return body.Data
}
