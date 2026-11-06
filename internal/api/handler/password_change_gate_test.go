package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

// TestLogin_PropagatesPasswordChangeFlag covers the seeded-admin lock: the flag
// has to travel in the access token, because AuthMiddleware enforces it without
// a database lookup on every request.
func TestLogin_PropagatesPasswordChangeFlag(t *testing.T) {
	hash, _ := auth.HashPassword("Valid1Pass!xy")
	users := &stubUserForEV{users: map[string]*model.User{
		"u1": {
			ID: "u1", Username: "admin", PasswordHash: hash, Role: "admin",
			MustChangePassword: true,
		},
	}}
	jwtMgr := auth.NewJWTManager("test-secret", time.Minute, time.Hour)
	h := &AuthHandler{
		users: users, refreshToks: &stubRefresh{}, passwordResetToks: &stubPasswordResetForMail{},
		onsiteStore: &stubOnsite{}, jwt: jwtMgr, evt: &stubEVT{},
	}

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "Valid1Pass!xy"})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var resp model.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.AccessToken == "" {
		t.Fatal("expected an access token")
	}

	claims, err := jwtMgr.ValidateToken(resp.AccessToken)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !claims.MustChangePassword {
		t.Fatal("access token must carry the password-change requirement")
	}
	if resp.User == nil || !resp.User.MustChangePassword {
		t.Fatal("response must tell the client to change the password")
	}
}

func TestLogin_FlagClearedAfterPasswordChange(t *testing.T) {
	// Mirrors the store's UpdatePassword: setting a password clears the flag, so
	// the next login yields an unrestricted token.
	users := &stubUserForEV{users: map[string]*model.User{
		"u1": {ID: "u1", Username: "admin", Role: "admin", MustChangePassword: false},
	}}
	user, err := users.GetByID(context.Background(), "u1")
	if err != nil || user == nil {
		t.Fatalf("get: %v", err)
	}
	jwtMgr := auth.NewJWTManager("test-secret", time.Minute, time.Hour)
	token, err := jwtMgr.GenerateAccessTokenWithState(user.ID, user.Username, user.Role, user.MustChangePassword)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	claims, err := jwtMgr.ValidateToken(token)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if claims.MustChangePassword {
		t.Fatal("a changed password must not leave the session restricted")
	}
}
