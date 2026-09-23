package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/tahsinarafat/aioj/internal/model"
)

type stubUserLookup struct {
	user *model.User
	err  error
}

func (s *stubUserLookup) GetByID(ctx context.Context, id string) (*model.User, error) {
	return s.user, s.err
}

type stubUserDeleter struct {
	deleted string
	err     error
}

func (s *stubUserDeleter) DeleteUserAndCascade(ctx context.Context, id string) error {
	if s.err != nil {
		return s.err
	}
	s.deleted = id
	return nil
}

func deletionHandler(t *testing.T, password string) (*UsersDeletionHandler, *stubUserDeleter) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	del := &stubUserDeleter{}
	return &UsersDeletionHandler{
		Users:   &stubUserLookup{user: &model.User{ID: "u1", Username: "alice", PasswordHash: string(hash)}},
		Deleter: del,
	}, del
}

func TestDeleteAccount_RequiresPasswordConfirmation(t *testing.T) {
	h, del := deletionHandler(t, "secret-pass")
	ctx := contextWithClaims(context.Background(), "u1")
	req := httptest.NewRequest("DELETE", "/api/users/me", strings.NewReader(`{"password":"wrong","confirm":true}`)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.DeleteAccount(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if del.deleted != "" {
		t.Error("must not delete on bad password")
	}
}

func TestDeleteAccount_RequiresConfirm(t *testing.T) {
	h, _ := deletionHandler(t, "secret-pass")
	ctx := contextWithClaims(context.Background(), "u1")
	req := httptest.NewRequest("DELETE", "/api/users/me", strings.NewReader(`{"password":"secret-pass","confirm":false}`)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.DeleteAccount(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 without confirm, got %d", rec.Code)
	}
}

func TestDeleteAccount_DeletesWithPasswordAndConfirm(t *testing.T) {
	h, del := deletionHandler(t, "secret-pass")
	ctx := contextWithClaims(context.Background(), "u1")
	req := httptest.NewRequest("DELETE", "/api/users/me", strings.NewReader(`{"password":"secret-pass","confirm":true}`)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.DeleteAccount(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if del.deleted != "u1" {
		t.Errorf("expected delete u1, got %q", del.deleted)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "deleted" {
		t.Errorf("unexpected body %v", body)
	}
}

func TestDeleteAccount_UnauthorizedWithoutClaims(t *testing.T) {
	h, _ := deletionHandler(t, "secret-pass")
	req := httptest.NewRequest("DELETE", "/api/users/me", strings.NewReader(`{"confirm":true}`))
	rec := httptest.NewRecorder()
	h.DeleteAccount(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
