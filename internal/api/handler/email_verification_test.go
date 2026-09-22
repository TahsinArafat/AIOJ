package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/model"
)

func TestEmailVerification_Verify_Success(t *testing.T) {
	us := &stubUserForEV{users: map[string]*model.User{"u1": {ID: "u1"}}}
	rawToken := "raw-token"
	sum := sha256.Sum256([]byte(rawToken))
	evt := &stubEVT{tok: &model.EmailVerificationToken{
		ID:        "tok1",
		UserID:    "u1",
		TokenHash: hex.EncodeToString(sum[:]),
		ExpiresAt: time.Now().Add(time.Hour),
	}}

	h := &EmailVerificationHandler{Users: us, Tokens: evt}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("token", rawToken)
	req := httptest.NewRequest("GET", "/api/auth/verify-email/"+rawToken, nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Verify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if us.marked != "u1" {
		t.Errorf("expected user u1 to be marked verified, got %q", us.marked)
	}
}

func TestEmailVerification_Verify_Expired(t *testing.T) {
	us := &stubUserForEV{users: map[string]*model.User{"u1": {ID: "u1"}}}
	rawToken := "expired"
	sum := sha256.Sum256([]byte(rawToken))
	evt := &stubEVT{tok: &model.EmailVerificationToken{
		ID:        "tok2",
		UserID:    "u1",
		TokenHash: hex.EncodeToString(sum[:]),
		ExpiresAt: time.Now().Add(-time.Minute),
	}}

	h := &EmailVerificationHandler{Users: us, Tokens: evt}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("token", rawToken)
	req := httptest.NewRequest("GET", "/x", nil).WithContext(
		context.WithValue(context.Background(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Verify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for expired token, got %d", rec.Code)
	}
	if us.marked != "" {
		t.Errorf("should not mark verified on expired token, marked %q", us.marked)
	}
}
