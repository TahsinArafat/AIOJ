package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type searchStub struct {
	problems []model.ProblemListItem
}

func (s *searchStub) SearchAll(_ context.Context, _ string, _ int) (*postgres.SearchResult, error) {
	return &postgres.SearchResult{Problems: s.problems}, nil
}

// Search is public and unauthenticated, and its JSON ships difficulty, rating and
// tags for every hit. The dropdown only renders titles, so this was a quiet way to
// ask "what difficulty is problem C of the running round?".
func TestSearchHidesDifficultyOnLiveContestProblems(t *testing.T) {
	stub := &searchStub{problems: []model.ProblemListItem{
		{Slug: "live", Title: "Live", Difficulty: "hard", Rating: 2400, Tags: []string{"dp"}, InUnreleasedContest: true},
		{Slug: "archived", Title: "Archived", Difficulty: "easy", Rating: 800, Tags: []string{"math"}},
	}}

	h := NewSearchHandler(stub)
	req := httptest.NewRequest("GET", "/api/search?q=pro", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	var body postgres.SearchResult
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	live := body.Problems[0]
	if live.Difficulty != "" || live.Rating != 0 || len(live.Tags) != 0 {
		t.Fatalf("live contest problem leaked: %+v", live)
	}
	if body.Problems[1].Difficulty != "easy" || len(body.Problems[1].Tags) == 0 {
		t.Fatalf("archived problem must keep its metadata: %+v", body.Problems[1])
	}
}
