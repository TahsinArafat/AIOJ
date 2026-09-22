package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/mail"
	"github.com/tahsinarafat/aioj/internal/model"
)

func TestForgotPassword_DoesNotLeakTokenInResponse(t *testing.T) {
	catcher := mail.NewMailCatcher()
	tpl, err := mail.LoadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	h := &AuthHandler{
		users: &stubUserForEV{users: map[string]*model.User{
			"u1": {ID: "u1", Username: "alice", Email: "alice@x.com"},
		}},
		passwordResetToks: &stubPasswordResetForMail{},
		mail:              catcher,
		mailTpl:           tpl,
		publicURL:         "http://localhost",
		mailFrom:          "noreply@aioj.com",
	}

	body := `{"email":"alice@x.com"}`
	req := httptest.NewRequest("POST", "/api/auth/forgot-password", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ForgotPassword(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if _, leaked := resp["token"]; leaked {
		t.Errorf("token leaked in response: %v", resp)
	}
	if len(catcher.All()) != 1 {
		t.Errorf("expected 1 email sent, got %d", len(catcher.All()))
	}
	if !strings.Contains(catcher.All()[0].Body, "alice") {
		t.Errorf("email body should contain username. got: %s", catcher.All()[0].Body)
	}
	if !strings.Contains(catcher.All()[0].Body, "reset-password?token=") {
		t.Errorf("email body should contain reset URL. got: %s", catcher.All()[0].Body)
	}
}

func TestRegister_SendsVerificationEmail(t *testing.T) {
	catcher := mail.NewMailCatcher()
	tpl, _ := mail.LoadTemplates()
	jwtMgr := auth.NewJWTManager("test-secret", time.Minute, 24*time.Hour)
	h := &AuthHandler{
		users:             &stubUserForEV{},
		refreshToks:       &stubRefresh{},
		passwordResetToks: &stubPasswordResetForMail{},
		evt:               &stubEVT{},
		mail:              catcher,
		mailTpl:           tpl,
		publicURL:         "http://localhost",
		mailFrom:          "noreply@aioj.com",
		jwt:               jwtMgr,
	}

	body := `{"username":"bob","email":"bob@x.com","password":"Valid1Pass!xy"}`
	req := httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if len(catcher.All()) != 1 {
		t.Fatalf("expected verification email, got %d", len(catcher.All()))
	}
	if !strings.Contains(catcher.All()[0].Body, "verify-email?token=") {
		t.Errorf("verification email missing verify URL. body: %s", catcher.All()[0].Body)
	}
}

func TestResendVerification_SendsEmailWhenUnverified(t *testing.T) {
	catcher := mail.NewMailCatcher()
	tpl, _ := mail.LoadTemplates()
	us := &stubUserForEV{
		users:      map[string]*model.User{"u1": {ID: "u1", Username: "alice", Email: "alice@x.com"}},
		isVerified: false,
	}
	h := &AuthHandler{
		users: us, refreshToks: &stubRefresh{}, evt: &stubEVT{},
		mail: catcher, mailTpl: tpl, publicURL: "http://localhost",
		mailFrom: "noreply@aioj.com",
	}
	req := httptest.NewRequest("POST", "/api/auth/verify-email/resend", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, &auth.Claims{UserID: "u1"}))
	rec := httptest.NewRecorder()
	h.ResendVerification(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if len(catcher.All()) != 1 {
		t.Fatalf("expected 1 email, got %d", len(catcher.All()))
	}
}

func TestResendVerification_AlreadyVerified_NoEmail(t *testing.T) {
	catcher := mail.NewMailCatcher()
	tpl, _ := mail.LoadTemplates()
	us := &stubUserForEV{
		users:      map[string]*model.User{"u1": {ID: "u1", Username: "alice", Email: "alice@x.com"}},
		isVerified: true,
	}
	h := &AuthHandler{
		users: us, refreshToks: &stubRefresh{}, evt: &stubEVT{},
		mail: catcher, mailTpl: tpl, publicURL: "http://localhost",
		mailFrom: "noreply@aioj.com",
	}
	req := httptest.NewRequest("POST", "/api/auth/verify-email/resend", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, &auth.Claims{UserID: "u1"}))
	rec := httptest.NewRecorder()
	h.ResendVerification(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if len(catcher.All()) != 0 {
		t.Errorf("expected no email for verified user, got %d", len(catcher.All()))
	}
}

type stubRefresh struct{}

func (s *stubRefresh) Create(context.Context, string, string, time.Time) error {
	return nil
}
func (s *stubRefresh) Validate(context.Context, string) (string, error) {
	return "", nil
}
