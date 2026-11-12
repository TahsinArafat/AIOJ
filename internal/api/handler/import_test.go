package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

type mockProblemStore struct {
	createErr error
	getBySlug *model.Problem
	// getByID answers GetByID, used by the access-guard tests.
	getByID *model.Problem
	// listItems is what List returns, so the spoiler tests can drive rows.
	listItems []model.ProblemListItem
	// unreleasedContest answers InUnreleasedContest for every problem.
	unreleasedContest bool
	// access maps "userID/problemID" -> access level. A nil map means nobody
	// has access anywhere, which is what the import tests rely on.
	access map[string]string
	// updated records the last problem handed to Update, so the review tests can
	// assert on what a handler tried to persist.
	updated *model.Problem
	// listStatus records the status a caller asked ListByStatus for.
	listStatus func(status string)
}

func (m *mockProblemStore) Create(_ context.Context, _ *model.Problem) error { return m.createErr }
func (m *mockProblemStore) GetByID(_ context.Context, _ string) (*model.Problem, error) {
	return m.getByID, nil
}
func (m *mockProblemStore) GetBySlug(_ context.Context, _ string) (*model.Problem, error) {
	return m.getBySlug, nil
}
func (m *mockProblemStore) List(_ context.Context, _, _ int, _ bool) ([]model.ProblemListItem, int, error) {
	return m.listItems, len(m.listItems), nil
}
func (m *mockProblemStore) ListWithFilter(_ context.Context, _, _ int, _ bool, _ string, _ []string, _ string, _ string, _ string, _ string) ([]model.ProblemListItem, int, error) {
	return nil, 0, nil
}
func (m *mockProblemStore) ListByCreatedBy(_ context.Context, _ string, _, _ int) ([]model.ProblemListItem, int, error) {
	return nil, 0, nil
}
func (m *mockProblemStore) ListSelectable(_ context.Context, _ string, _, _ int, _ string) ([]model.ProblemListItem, int, error) {
	return nil, 0, nil
}
func (m *mockProblemStore) GetAllTags(_ context.Context) ([]string, error) { return nil, nil }
func (m *mockProblemStore) InUnreleasedContest(_ context.Context, _ string) (bool, error) {
	return m.unreleasedContest, nil
}
func (m *mockProblemStore) UpdateCounts(_ context.Context, _ string, _, _ int) error { return nil }
func (m *mockProblemStore) Update(_ context.Context, _ string, p *model.Problem) error {
	m.updated = p
	return nil
}
func (m *mockProblemStore) Delete(_ context.Context, _ string) error                  { return nil }
func (m *mockProblemStore) SetReviewState(_ context.Context, _, _, _, _ string) error { return nil }
func (m *mockProblemStore) ListByStatus(_ context.Context, status string, _, _ int) ([]model.ProblemListItem, int, error) {
	if m.listStatus != nil {
		m.listStatus(status)
	}
	return nil, 0, nil
}
func (m *mockProblemStore) AddPermission(_ context.Context, _, _, _ string) error { return nil }
func (m *mockProblemStore) RemovePermission(_ context.Context, _, _ string) error { return nil }
func (m *mockProblemStore) GetPermissions(_ context.Context, _ string) ([]model.ProblemPermission, error) {
	return nil, nil
}
func (m *mockProblemStore) HasAccess(_ context.Context, problemID, userID string, required ...string) bool {
	level, ok := m.access[userID+"/"+problemID]
	if !ok {
		return false
	}
	for _, req := range required {
		if level == req {
			return true
		}
	}
	return false
}
func (m *mockProblemStore) ListAccessLevels(_ context.Context, userID string, problemIDs []string) (map[string]string, error) {
	out := make(map[string]string, len(problemIDs))
	for _, id := range problemIDs {
		if level, ok := m.access[userID+"/"+id]; ok {
			out[id] = level
		}
	}
	return out, nil
}
func (m *mockProblemStore) GetRecommendations(_ context.Context, _ string, _ int) (*model.RecommendationsResponse, error) {
	return nil, nil
}

func TestImportAtCoder_Authorization(t *testing.T) {
	tests := []struct {
		name       string
		claims     *auth.Claims
		wantStatus int
	}{
		{
			name:       "no claims - unauthorized",
			claims:     nil,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "user role - forbidden",
			claims:     &auth.Claims{UserID: "user1", Role: "user"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "admin role - allowed",
			claims:     &auth.Claims{UserID: "admin1", Role: "admin"},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "setter role - allowed",
			claims:     &auth.Claims{UserID: "setter1", Role: "setter"},
			wantStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockProblemStore{}
			h := NewImportHandler(mockStore, "/tmp/test")

			body, _ := json.Marshal(map[string]string{
				"contest_id": "abc300",
				"problem_id": "abc300_a",
			})
			req := httptest.NewRequest("POST", "/api/problems/import/atcoder", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			if tt.claims != nil {
				req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, tt.claims))
			}

			w := httptest.NewRecorder()
			h.ImportAtCoder(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("ImportAtCoder() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestImportAtCoder_InvalidRequest(t *testing.T) {
	mockStore := &mockProblemStore{}
	h := NewImportHandler(mockStore, "/tmp/test")

	claims := &auth.Claims{UserID: "admin1", Role: "admin"}

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
	}{
		{
			name:       "invalid JSON",
			body:       "invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing contest_id",
			body:       map[string]string{"problem_id": "abc300_a"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing problem_id",
			body:       map[string]string{"contest_id": "abc300"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty contest_id",
			body:       map[string]string{"contest_id": "", "problem_id": "abc300_a"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty problem_id",
			body:       map[string]string{"contest_id": "abc300", "problem_id": ""},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if s, ok := tt.body.(string); ok {
				body = []byte(s)
			} else {
				body, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest("POST", "/api/problems/import/atcoder", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))

			w := httptest.NewRecorder()
			h.ImportAtCoder(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("ImportAtCoder() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestImportToph_Authorization(t *testing.T) {
	tests := []struct {
		name       string
		claims     *auth.Claims
		wantStatus int
	}{
		{
			name:       "no claims - unauthorized",
			claims:     nil,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "user role - forbidden",
			claims:     &auth.Claims{UserID: "user1", Role: "user"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "admin role - allowed",
			claims:     &auth.Claims{UserID: "admin1", Role: "admin"},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "setter role - allowed",
			claims:     &auth.Claims{UserID: "setter1", Role: "setter"},
			wantStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockProblemStore{}
			h := NewImportHandler(mockStore, "/tmp/test")

			body, _ := json.Marshal(map[string]string{
				"problem_id": "hello-world-1",
			})
			req := httptest.NewRequest("POST", "/api/problems/import/toph", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			if tt.claims != nil {
				req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, tt.claims))
			}

			w := httptest.NewRecorder()
			h.ImportToph(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("ImportToph() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestImportToph_InvalidRequest(t *testing.T) {
	mockStore := &mockProblemStore{}
	h := NewImportHandler(mockStore, "/tmp/test")

	claims := &auth.Claims{UserID: "admin1", Role: "admin"}

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
	}{
		{
			name:       "invalid JSON",
			body:       "invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing problem_id",
			body:       map[string]string{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty problem_id",
			body:       map[string]string{"problem_id": ""},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if s, ok := tt.body.(string); ok {
				body = []byte(s)
			} else {
				body, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest("POST", "/api/problems/import/toph", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))

			w := httptest.NewRecorder()
			h.ImportToph(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("ImportToph() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestImportQOJ_Authorization(t *testing.T) {
	tests := []struct {
		name       string
		claims     *auth.Claims
		wantStatus int
	}{
		{
			name:       "no claims - unauthorized",
			claims:     nil,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "user role - forbidden",
			claims:     &auth.Claims{UserID: "user1", Role: "user"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "admin role - allowed",
			claims:     &auth.Claims{UserID: "admin1", Role: "admin"},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "setter role - allowed",
			claims:     &auth.Claims{UserID: "setter1", Role: "setter"},
			wantStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockProblemStore{}
			h := NewImportHandler(mockStore, "/tmp/test")

			body, _ := json.Marshal(map[string]string{
				"problem_id": "1001",
			})
			req := httptest.NewRequest("POST", "/api/problems/import/qoj", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			if tt.claims != nil {
				req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, tt.claims))
			}

			w := httptest.NewRecorder()
			h.ImportQOJ(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("ImportQOJ() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestImportQOJ_InvalidRequest(t *testing.T) {
	mockStore := &mockProblemStore{}
	h := NewImportHandler(mockStore, "/tmp/test")

	claims := &auth.Claims{UserID: "admin1", Role: "admin"}

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
	}{
		{
			name:       "invalid JSON",
			body:       "invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing problem_id",
			body:       map[string]string{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty problem_id",
			body:       map[string]string{"problem_id": ""},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if s, ok := tt.body.(string); ok {
				body = []byte(s)
			} else {
				body, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest("POST", "/api/problems/import/qoj", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, claims))

			w := httptest.NewRecorder()
			h.ImportQOJ(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("ImportQOJ() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}
