package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * Contest membership is a time-boxed grant.
 *
 * GetContestProblemByIndex deliberately skips p.visible so a private problem
 * still opens while its round is running — that is the point of attaching a
 * draft. But if the grant never expired, attaching a draft to any contest would
 * make it readable through that contest endpoint forever, permanently
 * side-stepping the fact that an unpublished problem is none of anyone's
 * business. These tests pin the expiry.
 */

func endedContest() *model.Contest {
	now := time.Now()
	return &model.Contest{ID: "c1", Visible: true, StartTime: now.Add(-2 * time.Hour), EndTime: now.Add(-time.Hour)}
}

func runningContest() *model.Contest {
	now := time.Now()
	return &model.Contest{ID: "c1", Visible: true, StartTime: now.Add(-time.Hour), EndTime: now.Add(time.Hour)}
}

func contestProblemStatus(t *testing.T, contest *model.Contest, problem *model.Problem, claims *auth.Claims) int {
	t.Helper()
	stub := &contestStub{contest: contest, problem: problem}
	store := &mockProblemStore{access: nil}
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
	return w.Code
}

// The whole point of the feature: a draft attached to a running round is readable.
func TestContestProblemOpensADraftDuringTheRound(t *testing.T) {
	code := contestProblemStatus(t, runningContest(), privateProblem(), &auth.Claims{UserID: "u", Role: "user"})

	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200: a draft must be readable while its contest runs", code)
	}
}

// ...and stops being readable once the round is over.
func TestContestProblemClosesADraftAfterTheRound(t *testing.T) {
	code := contestProblemStatus(t, endedContest(), privateProblem(), &auth.Claims{UserID: "u", Role: "user"})

	if code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: the contest grant must not outlive the round", code)
	}
}

func TestContestProblemKeepsADraftOpenForItsAuthorAfterTheRound(t *testing.T) {
	code := contestProblemStatus(t, endedContest(), privateProblem(), &auth.Claims{UserID: "author1", Role: "setter"})

	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for the problem's own author", code)
	}
}

func TestContestProblemKeepsAPublishedProblemOpenAfterTheRound(t *testing.T) {
	published := privateProblem()
	published.Visible = true

	code := contestProblemStatus(t, endedContest(), published, &auth.Claims{UserID: "u", Role: "user"})

	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200: a published problem is public after the round", code)
	}
}
