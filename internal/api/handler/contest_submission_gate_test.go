package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

/**
 * The reported exploit, pinned.
 *
 * A setter marked a round invite-only and password-protected. People who were never
 * invited opened it and submitted solutions. POST /api/submissions checked the time
 * window, registration and upsolving — every rule about whether a submitter is
 * eligible — and none about whether they were allowed in the round at all. The
 * password was never compared and group membership was never consulted, so a
 * contest_id was all an outsider needed.
 */

// gateContestStore answers the two questions the policy asks, per test case.
type gateContestStore struct {
	// embed the full interface so only the methods the test needs are implemented
	store.ContestStore
	contest   *model.Contest
	groupOK   bool
	groupErr  error
	staff     bool
	callCount int
}

func (s *gateContestStore) GetByID(context.Context, string) (*model.Contest, error) {
	return s.contest, nil
}

func (s *gateContestStore) HasAccess(context.Context, string, string, ...string) bool {
	return s.staff
}

func (s *gateContestStore) CheckGroupRestriction(context.Context, string, string) (bool, error) {
	s.callCount++
	return s.groupOK, s.groupErr
}

// recordingSubmitStore fails the test if a refused submission reaches storage.
type recordingSubmitStore struct {
	store.SubmissionStore
	created bool
}

func (s *recordingSubmitStore) Create(context.Context, *model.Submission) error {
	s.created = true
	return nil
}

func liveContest(hasPassword bool) *model.Contest {
	c := &model.Contest{
		ID: "c1", Visible: true,
		StartTime: time.Now().Add(-time.Hour), EndTime: time.Now().Add(time.Hour),
		CreatedBy: "creator1", UpsolvingEnabled: true,
	}
	if hasPassword {
		c.PasswordHash = testPasswordHash
	}
	return c
}

// submitContestProblem drives POST /api/submissions for a contest problem and
// reports the status plus whether the submission was actually stored.
func submitContestProblem(t *testing.T, contests *gateContestStore, granted bool, claims *auth.Claims) (int, bool) {
	t.Helper()

	subs := &recordingSubmitStore{}
	h := &SubmissionHandler{
		subStore:     subs,
		probStore:    &mockProblemStore{getByID: &model.Problem{ID: "p1", Source: "aioj"}},
		contestStore: contests,
		queue:        &nopQueue{},
	}
	h.access = newContestAccessPolicy(contests, grantStub{granted: granted})

	body, _ := json.Marshal(map[string]string{
		"problem_id": "p1", "language": "cpp", "source_code": "int main(){}",
		"contest_id": "c1",
	})

	req := httptest.NewRequest("POST", "/api/submissions", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{}))
	if claims != nil {
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))
	}
	w := httptest.NewRecorder()

	h.Create(w, req)

	return w.Code, subs.created
}

func TestSubmissionRefusedWithoutContestAccess(t *testing.T) {
	user := &auth.Claims{UserID: "outsider", Role: "user"}

	cases := []struct {
		name     string
		contest  *model.Contest
		groupOK  bool
		staff    bool
		granted  bool
		claims   *auth.Claims
		wantCode int
		wantSent bool
	}{
		{
			// The exact reported case: a live, password-protected round and a
			// contestant who never unlocked it.
			name: "password round without a grant is refused", contest: liveContest(true),
			groupOK: true, claims: user, wantCode: http.StatusForbidden,
		},
		{
			name: "password round with a grant is accepted", contest: liveContest(true),
			groupOK: true, granted: true, claims: user, wantCode: http.StatusCreated, wantSent: true,
		},
		{
			// The other half of the hole: group membership was never consulted here.
			name: "group non-member is refused", contest: liveContest(false),
			groupOK: false, claims: user, wantCode: http.StatusForbidden,
		},
		{
			name: "an unremarkable public round still accepts", contest: liveContest(false),
			groupOK: true, claims: user, wantCode: http.StatusCreated, wantSent: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			contests := &gateContestStore{contest: tc.contest, groupOK: tc.groupOK, staff: tc.staff}
			code, created := submitContestProblem(t, contests, tc.granted, tc.claims)

			if code != tc.wantCode {
				t.Fatalf("status = %d, want %d", code, tc.wantCode)
			}
			if created != tc.wantSent {
				t.Fatalf("submission stored = %v, want %v — a refused request must not reach the store", created, tc.wantSent)
			}
		})
	}
}

// A database failure while deciding access must refuse the submission, never allow
// it. This is the difference between a gate and a decoration.
func TestSubmissionFailsClosedOnAccessError(t *testing.T) {
	contests := &gateContestStore{
		contest:  liveContest(false),
		groupErr: context.DeadlineExceeded,
	}
	code, created := submitContestProblem(t, contests, false, &auth.Claims{UserID: "u", Role: "user"})

	if code == http.StatusCreated || created {
		t.Fatalf("status = %d, stored = %v; a store error must not become an access grant", code, created)
	}
}

// The password prompt has to be distinguishable from a flat refusal, or the contest
// page cannot tell a contestant to unlock the round rather than give up.
func TestSubmissionPasswordPromptIsDistinguishable(t *testing.T) {
	contests := &gateContestStore{contest: liveContest(true), groupOK: true}

	subs := &recordingSubmitStore{}
	h := &SubmissionHandler{
		subStore:     subs,
		probStore:    &mockProblemStore{getByID: &model.Problem{ID: "p1", Source: "aioj"}},
		contestStore: contests,
	}
	h.access = newContestAccessPolicy(contests, grantStub{})

	body, _ := json.Marshal(map[string]string{
		"problem_id": "p1", "language": "cpp", "source_code": "int main(){}",
		"contest_id": "c1",
	})
	req := httptest.NewRequest("POST", "/api/submissions", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{}))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, &auth.Claims{UserID: "u", Role: "user"}))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("password_required")) {
		t.Fatalf("body = %q, want it to name password_required so the UI can prompt", w.Body.String())
	}
}

// nopQueue swallows enqueues so an accepted submission can be driven end to end
// without a judge. Only the acceptance path reaches it; every refusal returns
// before the queue is touched.
type nopQueue struct{ enqueued int }

func (q *nopQueue) Enqueue(context.Context, string, int) error { q.enqueued++; return nil }
func (q *nopQueue) Dequeue(context.Context) (string, error)    { return "", nil }
func (q *nopQueue) Len() int                                   { return q.enqueued }
func (q *nopQueue) Close() error                               { return nil }
