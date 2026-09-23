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

type stubUserDataAggregator struct {
	user     *model.User
	profile  *model.UserProfile
	subCount int
}

func (s *stubUserDataAggregator) User(ctx context.Context, id string) (*model.User, error) {
	return s.user, nil
}
func (s *stubUserDataAggregator) Profile(ctx context.Context, id string) (*model.UserProfile, error) {
	return s.profile, nil
}
func (s *stubUserDataAggregator) SubmissionCount(ctx context.Context, id string) (int, error) {
	return s.subCount, nil
}

func contextWithClaims(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, middleware.UserContextKey, &auth.Claims{
		UserID:   userID,
		Username: "alice",
		Role:     "user",
	})
}

func TestExportMyData_ReturnsJSON(t *testing.T) {
	h := &UsersExportHandler{Data: &stubUserDataAggregator{
		user:     &model.User{ID: "u1", Username: "alice", Email: "a@x.com"},
		profile:  &model.UserProfile{UserID: "u1", Rating: 1500},
		subCount: 42,
	}}
	ctx := contextWithClaims(context.Background(), "u1")
	req := httptest.NewRequest("GET", "/api/users/me/export", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ExportMyData(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	user, _ := body["user"].(map[string]any)
	if user["username"] != "alice" {
		t.Errorf("expected username alice, got %v", body["user"])
	}
	if user["password_hash"] != nil {
		t.Errorf("password_hash must not appear in export")
	}
	if int(body["submission_count"].(float64)) != 42 {
		t.Errorf("expected 42 submissions, got %v", body["submission_count"])
	}
	if body["exported_at"] == nil {
		t.Error("missing exported_at")
	}
}

func TestExportMyData_UnauthorizedWithoutClaims(t *testing.T) {
	h := &UsersExportHandler{Data: &stubUserDataAggregator{}}
	req := httptest.NewRequest("GET", "/api/users/me/export", nil)
	rec := httptest.NewRecorder()
	h.ExportMyData(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
