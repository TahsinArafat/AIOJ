package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * Contest problems here are ordinary archive problems, so a problem that is live
 * in a contest used to be reachable straight from /api/problems/{slug}. Redacting
 * its fields left a public page with no difficulty, rating or tags and no reason
 * given — and still confirmed the problem existed.
 *
 * A live problem is now kept out of the archive entirely, the way Codeforces keeps
 * a round's problems out of the problem set until the round ends: the public page
 * answers 404 and the list and search never return it. The contest page is where it
 * lives meanwhile, with its difficulty withheld.
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

func getBySlugWith(t *testing.T, store *mockProblemStore, claims *auth.Claims) (int, *model.Problem) {
	t.Helper()
	h := NewProblemHandler(store)
	req := httptest.NewRequest("GET", "/api/problems/secret", nil)
	if claims != nil {
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))
	}
	w := httptest.NewRecorder()
	h.GetBySlug(w, req)

	if w.Code != http.StatusOK {
		return w.Code, nil
	}
	var got model.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return w.Code, &got
}

func TestGetBySlugHidesAProblemThatIsLiveInAContest(t *testing.T) {
	store := &mockProblemStore{getBySlug: spoiledProblem(), unreleasedContest: true}

	code, _ := getBySlugWith(t, store, nil)

	if code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 while the problem is in an unfinished contest", code)
	}
}

func TestGetBySlugServesTheProblemOnceTheContestEnds(t *testing.T) {
	store := &mockProblemStore{getBySlug: spoiledProblem(), unreleasedContest: false}

	code, got := getBySlugWith(t, store, nil)

	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 after the contest", code)
	}
	if got.Difficulty != "hard" || got.Rating != 2400 || len(got.Tags) == 0 {
		t.Fatalf("a problem that is back in the archive must carry its metadata: %+v", got)
	}
	if got.Description != "body" || got.Constraints == "" {
		t.Fatalf("statement went missing: %+v", got)
	}
}

// The authors and testers are not the audience the round measures, so hiding the
// problem from them would only stop them checking their own work.
func TestGetBySlugKeepsAccessForTheAuthor(t *testing.T) {
	store := &mockProblemStore{
		getBySlug:         spoiledProblem(),
		unreleasedContest: true,
		access:            map[string]string{"setter1/p1": "owner"},
	}

	code, got := getBySlugWith(t, store, &auth.Claims{UserID: "setter1", Role: "setter"})

	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for the author", code)
	}
	if got.Difficulty != "hard" || got.Rating != 2400 || len(got.Tags) == 0 {
		t.Fatalf("the author must keep the full view: %+v", got)
	}
}

func TestGetBySlugKeepsAccessForAdmins(t *testing.T) {
	store := &mockProblemStore{getBySlug: spoiledProblem(), unreleasedContest: true}

	code, got := getBySlugWith(t, store, &auth.Claims{UserID: "admin1", Role: "admin"})

	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for an admin", code)
	}
	if got.Difficulty != "hard" {
		t.Fatalf("an admin must keep the full view: %+v", got)
	}
}
