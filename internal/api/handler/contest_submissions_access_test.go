package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

/**
 * GET /api/contests/{id}/submissions?mine=false returns the whole contest's
 * traffic: every submitter's handle and verdict. The handler used to compute
 * isJudge, echo it in the response, and then never enforce it — so any signed-in
 * user could read the entire contest by appending the query parameter. The
 * manager UI needs that view, so the check has to be judge rights, not "nobody".
 */

type contestListerStub struct {
	store.ContestStore
	contest *model.Contest
	judge   bool
}

func (s contestListerStub) GetByID(_ context.Context, _ string) (*model.Contest, error) {
	return s.contest, nil
}
func (s contestListerStub) HasAccess(_ context.Context, _, _ string, _ ...string) bool {
	return s.judge
}

type contestSubmissionsStub struct {
	store.SubmissionStore
	// hit records whether the store was reached, so a refused request can be
	// shown to have stopped before the query rather than after.
	hit bool
}

func (s *contestSubmissionsStub) ListByContest(_ context.Context, _ string, _, _ int, _ model.SubmissionFilter) ([]model.Submission, int, error) {
	s.hit = true
	return []model.Submission{}, 0, nil
}

func listContestSubmissions(t *testing.T, judge bool, query string, claims *auth.Claims) (int, bool) {
	t.Helper()
	subs := &contestSubmissionsStub{}
	h := &SubmissionHandler{
		subStore:     subs,
		contestStore: contestListerStub{contest: &model.Contest{ID: "c1"}, judge: judge},
	}
	req := httptest.NewRequest("GET", "/api/contests/c1/submissions?"+query, nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{Keys: []string{"contestId"}, Values: []string{"c1"}},
	}))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))
	w := httptest.NewRecorder()

	h.ListByContest(w, req)

	return w.Code, subs.hit
}

func TestContestSubmissionsMineFalseNeedsJudgeRights(t *testing.T) {
	cases := []struct {
		name   string
		judge  bool
		claims *auth.Claims
		want   int
	}{
		{"contestant asking for everything", false, &auth.Claims{UserID: "u", Role: "user"}, http.StatusForbidden},
		{"setter who does not run this contest", false, &auth.Claims{UserID: "s", Role: "setter"}, http.StatusForbidden},
		{"the contest's judge", true, &auth.Claims{UserID: "j", Role: "setter"}, http.StatusOK},
		{"an admin", false, &auth.Claims{UserID: "a", Role: "admin"}, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, hit := listContestSubmissions(t, tc.judge, "mine=false", tc.claims)
			if code != tc.want {
				t.Fatalf("status = %d, want %d", code, tc.want)
			}
			if code != http.StatusOK && hit {
				t.Fatal("a refused request must not reach the store")
			}
		})
	}
}

// The participant's own view (`mine` unset, or true) stays open to everyone signed
// in, filtered to their own rows — that is what the submission page uses.
func TestContestSubmissionsOwnRowsStayOpen(t *testing.T) {
	for _, query := range []string{"", "mine=true"} {
		code, hit := listContestSubmissions(t, false, query, &auth.Claims{UserID: "u", Role: "user"})
		if code != http.StatusOK {
			t.Fatalf("%q: status = %d, want 200 for a participant's own rows", query, code)
		}
		if !hit {
			t.Fatalf("%q: the store should have been queried", query)
		}
	}
}
