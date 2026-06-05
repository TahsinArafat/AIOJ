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
}

func (m *mockProblemStore) Create(_ context.Context, _ *model.Problem) error { return m.createErr }
func (m *mockProblemStore) GetByID(_ context.Context, _ string) (*model.Problem, error) { return nil, nil }
func (m *mockProblemStore) GetBySlug(_ context.Context, _ string) (*model.Problem, error) { return m.getBySlug, nil }
func (m *mockProblemStore) List(_ context.Context, _, _ int) ([]model.ProblemListItem, int, error) { return nil, 0, nil }
func (m *mockProblemStore) ListWithFilter(_ context.Context, _, _ int, _ string, _ []string, _ string, _ string, _ string, _ string) ([]model.ProblemListItem, int, error) { return nil, 0, nil }
func (m *mockProblemStore) ListByCreatedBy(_ context.Context, _ string, _, _ int) ([]model.ProblemListItem, int, error) { return nil, 0, nil }
func (m *mockProblemStore) GetAllTags(_ context.Context) ([]string, error) { return nil, nil }
func (m *mockProblemStore) UpdateCounts(_ context.Context, _ string, _, _ int) error { return nil }
func (m *mockProblemStore) Update(_ context.Context, _ string, _ *model.Problem) error { return nil }
func (m *mockProblemStore) Delete(_ context.Context, _ string) error { return nil }
func (m *mockProblemStore) AddPermission(_ context.Context, _, _, _ string) error { return nil }
func (m *mockProblemStore) RemovePermission(_ context.Context, _, _ string) error { return nil }
func (m *mockProblemStore) GetPermissions(_ context.Context, _ string) ([]model.ProblemPermission, error) { return nil, nil }
func (m *mockProblemStore) HasAccess(_ context.Context, _, _ string, _ ...string) bool { return false }
func (m *mockProblemStore) GetRecommendations(_ context.Context, _ string, _ int) (*model.RecommendationsResponse, error) { return nil, nil }

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
