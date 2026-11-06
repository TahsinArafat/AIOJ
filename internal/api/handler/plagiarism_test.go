package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/plagiarism"
	"github.com/tahsinarafat/aioj/internal/store"
)

// --- stubs ---------------------------------------------------------------

type plagiarismStoreStub struct {
	report    *model.PlagiarismReport
	pair      *model.PlagiarismPair
	created   int
	updated   []model.PlagiarismPairStatus
	createErr error
}

func (s *plagiarismStoreStub) CreateReport(_ context.Context, r *model.PlagiarismReport) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.created++
	r.ID = "report-1"
	return nil
}
func (s *plagiarismStoreStub) GetReportByID(context.Context, string) (*model.PlagiarismReport, error) {
	return s.report, nil
}
func (s *plagiarismStoreStub) GetReportByContest(context.Context, string) (*model.PlagiarismReport, error) {
	return s.report, nil
}
func (s *plagiarismStoreStub) UpdateReportStatus(context.Context, string, model.PlagiarismReportStatus, string) error {
	return nil
}
func (s *plagiarismStoreStub) UpdateReportCounts(context.Context, string, int, int) error {
	return nil
}
func (s *plagiarismStoreStub) CreatePair(context.Context, *model.PlagiarismPair) error { return nil }
func (s *plagiarismStoreStub) GetPairByID(context.Context, string) (*model.PlagiarismPair, error) {
	return s.pair, nil
}
func (s *plagiarismStoreStub) ListPairsByReport(context.Context, string, int, int) ([]model.PlagiarismPairDetail, int, error) {
	return nil, 0, nil
}
func (s *plagiarismStoreStub) UpdatePairStatus(_ context.Context, _ string, status model.PlagiarismPairStatus) error {
	s.updated = append(s.updated, status)
	return nil
}
func (s *plagiarismStoreStub) GetPairsByUser(context.Context, string) ([]model.PlagiarismPairDetail, error) {
	return nil, nil
}

// plagiarismContestStub resolves one contest and answers permission queries.
// The embedded interface supplies the methods these handlers never call.
type plagiarismContestStub struct {
	store.ContestStore
	contest *model.Contest
	access  bool
}

func (s *plagiarismContestStub) GetByID(_ context.Context, id string) (*model.Contest, error) {
	if s.contest == nil || s.contest.ID != id {
		return nil, nil
	}
	return s.contest, nil
}

func (s *plagiarismContestStub) HasAccess(_ context.Context, _, _ string, _ ...string) bool {
	return s.access
}

func (s *plagiarismContestStub) GetProblems(context.Context, string) ([]model.ContestProblem, error) {
	return nil, nil
}

// --- helpers -------------------------------------------------------------

func plagiarismRequest(t *testing.T, method, path string, params map[string]string, claims *auth.Claims, body any) *http.Request {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}

	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	req := httptest.NewRequest(method, path, reader)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	if claims != nil {
		ctx = context.WithValue(ctx, middleware.UserContextKey, claims)
	}
	return req.WithContext(ctx)
}

func newPlagiarismHandler(t *testing.T, store *plagiarismStoreStub, contests *plagiarismContestStub) *PlagiarismHandler {
	t.Helper()
	return NewPlagiarismHandler(nil, store, contests)
}

func contestFixture() *model.Contest {
	return &model.Contest{ID: "contest-1", CreatedBy: "owner-1"}
}

// --- tests ---------------------------------------------------------------

func TestPlagiarism_RunCheckRejectsNonStaff(t *testing.T) {
	store := &plagiarismStoreStub{}
	contests := &plagiarismContestStub{contest: contestFixture(), access: false}
	h := newPlagiarismHandler(t, store, contests)

	rec := httptest.NewRecorder()
	h.RunCheck(rec, plagiarismRequest(t, http.MethodPost, "/api/contests/contest-1/plagiarism/check", nil,
		&auth.Claims{UserID: "random-user", Role: "user"},
		map[string]any{"contest_id": "contest-1"}))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if store.created != 0 {
		t.Fatal("a non-staff caller must not create a report")
	}
}

func TestPlagiarism_RunCheckAllowsAdminAndCreatesReport(t *testing.T) {
	store := &plagiarismStoreStub{}
	contests := &plagiarismContestStub{contest: contestFixture(), access: false}
	// The stub contest returns no problems, so the scan finishes immediately.
	h := NewPlagiarismHandler(plagiarism.NewService(store, contests, nil), store, contests)

	rec := httptest.NewRecorder()
	h.RunCheck(rec, plagiarismRequest(t, http.MethodPost, "/api/contests/contest-1/plagiarism/check", nil,
		&auth.Claims{UserID: "admin-1", Role: "admin"},
		map[string]any{"contest_id": "contest-1"}))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body.String())
	}
	if store.created != 1 {
		t.Fatalf("reports created = %d, want 1", store.created)
	}
}

func TestPlagiarism_RunCheckRejectsConcurrentScan(t *testing.T) {
	store := &plagiarismStoreStub{}
	contests := &plagiarismContestStub{contest: contestFixture(), access: true}
	h := NewPlagiarismHandler(plagiarism.NewService(store, contests, nil), store, contests)

	// Occupy every slot the handler allows.
	for i := 0; i < maxConcurrentPlagiarismChecks; i++ {
		h.sem <- struct{}{}
	}
	defer func() {
		for i := 0; i < maxConcurrentPlagiarismChecks; i++ {
			<-h.sem
		}
	}()

	rec := httptest.NewRecorder()
	h.RunCheck(rec, plagiarismRequest(t, http.MethodPost, "/api/contests/contest-1/plagiarism/check", nil,
		&auth.Claims{UserID: "manager-1", Role: "user"},
		map[string]any{"contest_id": "contest-1"}))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429: %s", rec.Code, rec.Body.String())
	}
	if store.created != 0 {
		t.Fatal("no report should be created when the scan is refused")
	}
}

func TestPlagiarism_RunCheckRejectsBadThreshold(t *testing.T) {
	store := &plagiarismStoreStub{}
	contests := &plagiarismContestStub{contest: contestFixture(), access: true}
	h := NewPlagiarismHandler(plagiarism.NewService(store, contests, nil), store, contests)

	rec := httptest.NewRecorder()
	h.RunCheck(rec, plagiarismRequest(t, http.MethodPost, "/api/contests/contest-1/plagiarism/check", nil,
		&auth.Claims{UserID: "manager-1", Role: "user"},
		map[string]any{"contest_id": "contest-1", "threshold": 4.2}))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestPlagiarism_RunCheckAllowsContestOwnerAndManager(t *testing.T) {
	for _, tc := range []struct {
		name   string
		claims *auth.Claims
		access bool
	}{
		{"creator", &auth.Claims{UserID: "owner-1", Role: "user"}, false},
		{"manager", &auth.Claims{UserID: "manager-1", Role: "user"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			contests := &plagiarismContestStub{contest: contestFixture(), access: tc.access}
			h := newPlagiarismHandler(t, &plagiarismStoreStub{}, contests)

			// Exercise the authorization helper directly: starting a real scan
			// would spawn a goroutine against a stub service.
			rec := httptest.NewRecorder()
			req := plagiarismRequest(t, http.MethodPost, "/api/contests/contest-1/plagiarism/check", nil, tc.claims, nil)
			if _, ok := h.requireContestStaff(rec, req, "contest-1"); !ok {
				t.Fatalf("expected staff access, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestPlagiarism_RequireContestStaffRejectsAnonymous(t *testing.T) {
	h := newPlagiarismHandler(t, &plagiarismStoreStub{}, &plagiarismContestStub{contest: contestFixture()})

	rec := httptest.NewRecorder()
	req := plagiarismRequest(t, http.MethodGet, "/api/contests/contest-1/plagiarism/report", nil, nil, nil)
	if _, ok := h.requireContestStaff(rec, req, "contest-1"); ok {
		t.Fatal("anonymous caller must be rejected")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestPlagiarism_ReportReadRequiresStaff(t *testing.T) {
	store := &plagiarismStoreStub{report: &model.PlagiarismReport{ID: "report-1", ContestID: "contest-1"}}
	contests := &plagiarismContestStub{contest: contestFixture(), access: false}
	h := newPlagiarismHandler(t, store, contests)

	rec := httptest.NewRecorder()
	h.GetReport(rec, plagiarismRequest(t, http.MethodGet, "/api/contests/contest-1/plagiarism/report/report-1",
		map[string]string{"reportId": "report-1"}, &auth.Claims{UserID: "random-user", Role: "user"}, nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
}

func TestPlagiarism_UpdatePairStatusRequiresStaffAndValidStatus(t *testing.T) {
	store := &plagiarismStoreStub{
		report: &model.PlagiarismReport{ID: "report-1", ContestID: "contest-1"},
		pair:   &model.PlagiarismPair{ID: "pair-1", ReportID: "report-1"},
	}
	contests := &plagiarismContestStub{contest: contestFixture(), access: false}
	h := newPlagiarismHandler(t, store, contests)
	params := map[string]string{"pairId": "pair-1"}

	// Unprivileged caller: rejected, and nothing is written.
	rec := httptest.NewRecorder()
	h.UpdatePairStatus(rec, plagiarismRequest(t, http.MethodPut, "/api/contests/contest-1/plagiarism/pairs/pair-1",
		params, &auth.Claims{UserID: "random-user", Role: "user"},
		map[string]string{"status": string(model.PlagiarismPairIgnored)}))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if len(store.updated) != 0 {
		t.Fatalf("pair status written by a non-staff caller: %v", store.updated)
	}

	// Staff caller with a bogus status: rejected before any write.
	contests.access = true
	rec = httptest.NewRecorder()
	h.UpdatePairStatus(rec, plagiarismRequest(t, http.MethodPut, "/api/contests/contest-1/plagiarism/pairs/pair-1",
		params, &auth.Claims{UserID: "manager-1", Role: "user"},
		map[string]string{"status": "whatever"}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if len(store.updated) != 0 {
		t.Fatal("an invalid status must not be persisted")
	}

	// Staff caller with a valid status: accepted.
	rec = httptest.NewRecorder()
	h.UpdatePairStatus(rec, plagiarismRequest(t, http.MethodPut, "/api/contests/contest-1/plagiarism/pairs/pair-1",
		params, &auth.Claims{UserID: "manager-1", Role: "user"},
		map[string]string{"status": string(model.PlagiarismPairIgnored)}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(store.updated) != 1 || store.updated[0] != model.PlagiarismPairIgnored {
		t.Fatalf("updated = %v, want [ignored]", store.updated)
	}
}
