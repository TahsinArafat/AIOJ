package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type stubTOTP struct {
	secret  *model.TOTPSecret
	upsert  string
	enabled bool
}

func (s *stubTOTP) Upsert(_ context.Context, _, secret string) error {
	s.upsert = secret
	s.secret = &model.TOTPSecret{UserID: "u1", Secret: secret, Enabled: false}
	return nil
}
func (s *stubTOTP) Enable(_ context.Context, _ string) error {
	if s.secret != nil {
		s.secret.Enabled = true
		now := time.Now()
		s.secret.EnabledAt = &now
	}
	s.enabled = true
	return nil
}
func (s *stubTOTP) Disable(_ context.Context, _ string) error {
	s.secret = nil
	s.enabled = false
	return nil
}
func (s *stubTOTP) Get(context.Context, string) (*model.TOTPSecret, error) {
	return s.secret, nil
}

type stubBackups struct {
	codes map[string]string // id -> hash
	used  map[string]bool
}

func (s *stubBackups) Create(_ context.Context, id, _, hash string) error {
	if s.codes == nil {
		s.codes = map[string]string{}
	}
	s.codes[id] = hash
	return nil
}
func (s *stubBackups) ListActive(context.Context, string) ([]store.BackupCodeRow, error) {
	var out []store.BackupCodeRow
	for id, h := range s.codes {
		if !s.used[id] {
			out = append(out, store.BackupCodeRow{ID: id, Hash: h})
		}
	}
	return out, nil
}
func (s *stubBackups) Consume(_ context.Context, id string) error {
	if s.used == nil {
		s.used = map[string]bool{}
	}
	s.used[id] = true
	return nil
}

func TestTwoFactor_BeginUpsertsSecret(t *testing.T) {
	us := &stubUserForEV{users: map[string]*model.User{
		"u1": {ID: "u1", Username: "alice", Email: "alice@x.com"},
	}}
	totp := &stubTOTP{}
	h := &TwoFactorHandler{Users: us, Secrets: totp, Backups: &stubBackups{}}

	req := httptest.NewRequest("POST", "/api/auth/2fa/begin", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, &auth.Claims{UserID: "u1"}))
	rec := httptest.NewRecorder()
	h.Begin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if totp.upsert == "" {
		t.Fatal("expected secret upserted")
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["secret"] != totp.upsert {
		t.Errorf("response secret mismatch")
	}
}

func TestTwoFactor_EnableRequiresValidCode(t *testing.T) {
	secret, _ := auth.GenerateTOTPSecret()
	totp := &stubTOTP{secret: &model.TOTPSecret{UserID: "u1", Secret: secret, Enabled: false}}
	us := &stubUserForEV{users: map[string]*model.User{"u1": {ID: "u1"}}}
	h := &TwoFactorHandler{Users: us, Secrets: totp, Backups: &stubBackups{}}

	code, _ := auth.GenerateTOTPCodeForTest(secret)
	payload, _ := json.Marshal(map[string]string{"code": code})
	req := httptest.NewRequest("POST", "/api/auth/2fa/enable", bytes.NewReader(payload))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, &auth.Claims{UserID: "u1"}))
	rec := httptest.NewRecorder()
	h.Enable(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		BackupCodes []string `json:"backup_codes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.BackupCodes) != 10 {
		t.Errorf("expected 10 backup codes, got %d", len(body.BackupCodes))
	}
}

func TestLogin_Returns2FAChallengeWhenEnabled(t *testing.T) {
	hash, _ := auth.HashPassword("Valid1Pass!xy")
	us := &stubUserForEV{users: map[string]*model.User{
		"u1": {ID: "u1", Username: "alice", PasswordHash: hash, Role: "user"},
	}}
	totp := &stubTOTP{secret: &model.TOTPSecret{UserID: "u1", Secret: "MFRGGZDFMZTWQ2LK", Enabled: true}, enabled: true}
	jwtMgr := auth.NewJWTManager("test-secret", time.Minute, 24*time.Hour)
	h := &AuthHandler{
		users: us, refreshToks: &stubRefresh{}, passwordResetToks: &stubPasswordResetForMail{},
		onsiteStore: &stubOnsite{}, contestStore: nil,
		jwt: jwtMgr, evt: &stubEVT{}, twoFA: totp,
	}
	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "Valid1Pass!xy"})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var resp model.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Requires2FA || resp.ChallengeID == "" || resp.AccessToken != "" {
		t.Fatalf("expected challenge only, got %+v", resp)
	}
	if _, err := jwtMgr.ParseChallengeToken(resp.ChallengeID); err != nil {
		t.Fatalf("challenge token invalid: %v", err)
	}
}

// stubOnsite is a no-op OnsiteUserStore for login tests.
type stubOnsite struct{}

func (s *stubOnsite) CreateBatch(context.Context, string, []model.BatchUserRequest) ([]model.OnsiteBatchUser, error) {
	return nil, nil
}
func (s *stubOnsite) GetByUsername(context.Context, string) (*model.OnsiteBatchUser, error) {
	return nil, nil
}
func (s *stubOnsite) MarkUsed(context.Context, string, string) error { return nil }
func (s *stubOnsite) ListByContest(context.Context, string) ([]model.OnsiteBatchUser, error) {
	return nil, nil
}
func (s *stubOnsite) DeleteByContest(context.Context, string) error { return nil }
func (s *stubOnsite) DeleteByID(context.Context, string) error      { return nil }
func (s *stubOnsite) AutoRegister(context.Context, string, string) error {
	return nil
}
