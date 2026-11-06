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

// onsiteLoginStub returns one pre-seeded onsite credential.
type onsiteLoginStub struct {
	user *model.OnsiteBatchUser
}

func (s *onsiteLoginStub) CreateBatch(context.Context, string, []model.BatchUserRequest) ([]model.OnsiteBatchUser, error) {
	return nil, nil
}
func (s *onsiteLoginStub) GetByUsername(context.Context, string) (*model.OnsiteBatchUser, error) {
	return s.user, nil
}
func (s *onsiteLoginStub) MarkUsed(context.Context, string, string) error { return nil }
func (s *onsiteLoginStub) ListByContest(context.Context, string) ([]model.OnsiteBatchUser, error) {
	return nil, nil
}
func (s *onsiteLoginStub) DeleteByContest(context.Context, string) error { return nil }
func (s *onsiteLoginStub) DeleteByID(context.Context, string) error      { return nil }
func (s *onsiteLoginStub) AutoRegister(context.Context, string, string) error {
	return nil
}

// countingRefreshStore records issued refresh tokens so a test can assert that
// none were minted when the second factor is still outstanding.
type countingRefreshStore struct {
	created int
}

func (s *countingRefreshStore) Create(context.Context, string, string, time.Time) error {
	s.created++
	return nil
}
func (s *countingRefreshStore) Validate(context.Context, string) (string, error) { return "", nil }

func onsiteLoginRequest(t *testing.T, username, password string) *http.Request {
	t.Helper()
	body, err := json.Marshal(map[string]string{"username": username, "password": password})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return httptest.NewRequest(http.MethodPost, "/api/contests/1/onsite/login", bytes.NewReader(body))
}

// TestOnsiteLogin_RequiresSecondFactor is the regression test for the 2FA
// bypass: this route issued a full session from the password alone.
func TestOnsiteLogin_RequiresSecondFactor(t *testing.T) {
	hash, _ := auth.HashPassword("TeamPass!2026")
	linked := "u1"

	onsite := &onsiteLoginStub{user: &model.OnsiteBatchUser{
		ID: "o1", ContestID: "c1", Username: "team-01",
		PasswordHash: hash, IsUsed: true, UsedBy: &linked,
	}}
	users := &stubUserForEV{users: map[string]*model.User{
		"u1": {ID: "u1", Username: "team-01", Role: "contestant"},
	}}
	refresh := &countingRefreshStore{}
	totp := &stubTOTP{secret: &model.TOTPSecret{UserID: "u1", Secret: "MFRGGZDFMZTWQ2LK", Enabled: true}}

	h := NewOnsiteBatchHandler(nil, onsite, users, refresh, totp, auth.NewJWTManager("test-secret", time.Minute, time.Hour))

	rec := httptest.NewRecorder()
	h.LoginAsTeam(rec, onsiteLoginRequest(t, "team-01", "TeamPass!2026"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var resp model.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Requires2FA || resp.ChallengeID == "" {
		t.Fatalf("expected a 2FA challenge, got %+v", resp)
	}
	if resp.AccessToken != "" || resp.RefreshToken != "" {
		t.Fatal("no session tokens may be issued before the second factor")
	}
	if refresh.created != 0 {
		t.Fatalf("issued %d refresh tokens before 2FA completed, want 0", refresh.created)
	}

	// The challenge must not itself be usable as a session token.
	if _, err := h.jwtManager.ValidateToken(resp.ChallengeID); err == nil {
		t.Fatal("challenge token must not validate as an access token")
	}
}

func TestOnsiteLogin_IssuesTokensWithout2FA(t *testing.T) {
	hash, _ := auth.HashPassword("TeamPass!2026")
	linked := "u1"

	onsite := &onsiteLoginStub{user: &model.OnsiteBatchUser{
		ID: "o1", ContestID: "c1", Username: "team-02",
		PasswordHash: hash, IsUsed: true, UsedBy: &linked,
	}}
	users := &stubUserForEV{users: map[string]*model.User{
		"u1": {ID: "u1", Username: "team-02", Role: "contestant"},
	}}
	refresh := &countingRefreshStore{}
	// TOTP enrolled but never enabled: no challenge.
	totp := &stubTOTP{secret: &model.TOTPSecret{UserID: "u1", Secret: "MFRGGZDFMZTWQ2LK", Enabled: false}}

	h := NewOnsiteBatchHandler(nil, onsite, users, refresh, totp, auth.NewJWTManager("test-secret", time.Minute, time.Hour))

	rec := httptest.NewRecorder()
	h.LoginAsTeam(rec, onsiteLoginRequest(t, "team-02", "TeamPass!2026"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var resp model.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Requires2FA {
		t.Fatal("did not expect a 2FA challenge for an account without TOTP")
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatal("expected session tokens")
	}
	if refresh.created != 1 {
		t.Fatalf("refresh tokens created = %d, want 1", refresh.created)
	}
}

func TestOnsiteLogin_RejectsBadPassword(t *testing.T) {
	hash, _ := auth.HashPassword("TeamPass!2026")
	onsite := &onsiteLoginStub{user: &model.OnsiteBatchUser{
		ID: "o1", Username: "team-03", PasswordHash: hash,
	}}
	h := NewOnsiteBatchHandler(nil, onsite, &stubUserForEV{}, &countingRefreshStore{}, nil,
		auth.NewJWTManager("test-secret", time.Minute, time.Hour))

	rec := httptest.NewRecorder()
	h.LoginAsTeam(rec, onsiteLoginRequest(t, "team-03", "wrong"))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
