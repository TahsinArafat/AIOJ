package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

func TestSubmission_Create_RejectsUnverifiedEmail(t *testing.T) {
	h := &SubmissionHandler{
		users: &stubUserForEV{
			users:      map[string]*model.User{"u1": {ID: "u1", Email: "a@x.com"}},
			isVerified: false,
		},
	}
	req := httptest.NewRequest("POST", "/api/submissions", strings.NewReader(`{"problem_id":"p","language":"c","source_code":"x"}`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, &auth.Claims{UserID: "u1"}))
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}
