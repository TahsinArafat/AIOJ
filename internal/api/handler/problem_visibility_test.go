package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

// A setter publishing straight to the archive was the behaviour this replaced.
// Only an admin may create a problem that is already published; everybody else
// starts as a draft and reaches the archive through the review queue.
func TestInitialStatus(t *testing.T) {
	cases := []struct {
		name      string
		requested bool
		role      string
		want      string
	}{
		{name: "admin can publish directly", requested: true, role: "admin", want: model.ProblemStatusPublished},
		{name: "admin can keep it a draft", requested: false, role: "admin", want: model.ProblemStatusDraft},
		{name: "a setter cannot publish", requested: true, role: "setter", want: model.ProblemStatusDraft},
		{name: "a setter can keep it a draft", requested: false, role: "setter", want: model.ProblemStatusDraft},
		{name: "an ordinary user cannot publish", requested: true, role: "user", want: model.ProblemStatusDraft},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := initialStatus(tc.requested, tc.role); got != tc.want {
				t.Fatalf("initialStatus(%v, %q) = %q, want %q", tc.requested, tc.role, got, tc.want)
			}
		})
	}
}

// recordingProblemStore records the admin flag List was called with, so a test
// can assert the handler widened the archive for an admin and only for an admin.
type recordingProblemStore struct {
	mockProblemStore
	adminFlag bool
}

func (m *recordingProblemStore) List(_ context.Context, _, _ int, admin bool) ([]model.ProblemListItem, int, error) {
	m.adminFlag = admin
	return m.listItems, len(m.listItems), nil
}

// An admin is the one person who approves problems, so the archive listing must
// include the drafts still waiting. Every other caller keeps the published-only
// archive.
func TestListArchiveWidenstoAdmin(t *testing.T) {
	cases := []struct {
		name      string
		claims    *auth.Claims
		wantAdmin bool
	}{
		{name: "anonymous keeps the public archive", claims: nil, wantAdmin: false},
		{name: "ordinary user keeps the public archive", claims: &auth.Claims{UserID: "u1", Role: "user"}, wantAdmin: false},
		{name: "setter keeps the public archive", claims: &auth.Claims{UserID: "s1", Role: "setter"}, wantAdmin: false},
		{name: "admin sees drafts too", claims: &auth.Claims{UserID: "a1", Role: "admin"}, wantAdmin: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &recordingProblemStore{}
			h := NewProblemHandler(store)
			req := httptest.NewRequest("GET", "/api/problems", nil)
			if tc.claims != nil {
				req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, tc.claims))
			}
			w := httptest.NewRecorder()
			h.List(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}
			if store.adminFlag != tc.wantAdmin {
				t.Fatalf("admin flag = %v, want %v", store.adminFlag, tc.wantAdmin)
			}
		})
	}
}
