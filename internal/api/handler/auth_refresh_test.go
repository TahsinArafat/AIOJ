package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

// memoryRefreshStore models the real store's rotation contract: a token can be
// consumed exactly once, and RevokeAll drops everything for a user.
type memoryRefreshStore struct {
	mu      sync.Mutex
	tokens  map[string]string // hash -> userID
	revoked int
}

func newMemoryRefreshStore() *memoryRefreshStore {
	return &memoryRefreshStore{tokens: map[string]string{}}
}

func (s *memoryRefreshStore) Create(_ context.Context, userID, hash string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[hash] = userID
	return nil
}

func (s *memoryRefreshStore) Consume(_ context.Context, hash string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	userID, ok := s.tokens[hash]
	if !ok {
		return "", context.Canceled // any error; callers only check err != nil
	}
	delete(s.tokens, hash)
	return userID, nil
}

func (s *memoryRefreshStore) RevokeAll(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for hash, owner := range s.tokens {
		if owner == userID {
			delete(s.tokens, hash)
		}
	}
	s.revoked++
	return nil
}

func (s *memoryRefreshStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.tokens)
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// validResetTokenStore hands back an unused, unexpired reset token.
type validResetTokenStore struct {
	userID string
	used   bool
}

func (s *validResetTokenStore) Create(context.Context, string, string, string, time.Time) error {
	return nil
}
func (s *validResetTokenStore) GetByHash(context.Context, string) (*model.PasswordResetToken, error) {
	return &model.PasswordResetToken{
		ID: "reset-1", UserID: s.userID,
		ExpiresAt: time.Now().Add(time.Hour), Used: s.used,
	}, nil
}
func (s *validResetTokenStore) MarkUsed(context.Context, string) error {
	s.used = true
	return nil
}

func newAuthHandlerWithStore(t *testing.T, store *memoryRefreshStore) *AuthHandler {
	t.Helper()
	users := &stubUserForEV{users: map[string]*model.User{
		"u1": {ID: "u1", Username: "alice", Role: "user"},
	}}
	return &AuthHandler{
		users:       users,
		refreshToks: store,
		jwt:         auth.NewJWTManager("test-secret", time.Minute, time.Hour),
	}
}

func postJSON(t *testing.T, h http.HandlerFunc, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

// TestRefresh_RotatesToken covers the replay hole: the old token used to stay
// valid after a refresh, so a stolen copy worked for the full 168h TTL.
func TestRefresh_RotatesToken(t *testing.T) {
	store := newMemoryRefreshStore()
	h := newAuthHandlerWithStore(t, store)

	const raw = "original-refresh-token"
	if err := store.Create(context.Background(), "u1", hashToken(raw), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := postJSON(t, h.Refresh, "/api/auth/refresh", map[string]string{"refresh_token": raw})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var resp model.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.RefreshToken == "" || resp.RefreshToken == raw {
		t.Fatal("expected a rotated refresh token")
	}
	if _, err := store.Consume(context.Background(), hashToken(raw)); err == nil {
		t.Fatal("the presented token must be spent")
	}
	if store.count() != 1 {
		t.Fatalf("live refresh tokens = %d, want 1", store.count())
	}

	// Replaying the old token must fail.
	rec = postJSON(t, h.Refresh, "/api/auth/refresh", map[string]string{"refresh_token": raw})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("replay status = %d, want 401", rec.Code)
	}
}

func TestRefresh_RejectsUnknownToken(t *testing.T) {
	store := newMemoryRefreshStore()
	h := newAuthHandlerWithStore(t, store)

	rec := postJSON(t, h.Refresh, "/api/auth/refresh", map[string]string{"refresh_token": "nope"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestLogout_RevokesTokenIdempotently(t *testing.T) {
	store := newMemoryRefreshStore()
	h := newAuthHandlerWithStore(t, store)

	const raw = "session-token"
	_ = store.Create(context.Background(), "u1", hashToken(raw), time.Now().Add(time.Hour))

	rec := postJSON(t, h.Logout, "/api/auth/logout", map[string]string{"refresh_token": raw})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if store.count() != 0 {
		t.Fatalf("refresh tokens left = %d, want 0", store.count())
	}

	// Logging out twice (or with a bogus token) must still succeed.
	for _, tok := range []string{raw, "bogus"} {
		rec = postJSON(t, h.Logout, "/api/auth/logout", map[string]string{"refresh_token": tok})
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
	}

	// And the revoked token can no longer be refreshed.
	rec = postJSON(t, h.Refresh, "/api/auth/refresh", map[string]string{"refresh_token": raw})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout = %d, want 401", rec.Code)
	}
}

// TestResetPassword_RevokesAllSessions drives the real handler: a reset is how
// a compromised account is recovered, so it must kill the attacker's sessions.
func TestResetPassword_RevokesAllSessions(t *testing.T) {
	store := newMemoryRefreshStore()
	h := newAuthHandlerWithStore(t, store)
	h.passwordResetToks = &validResetTokenStore{userID: "u1"}

	// Two live sessions for the same user, plus one for somebody else.
	_ = store.Create(context.Background(), "u1", hashToken("laptop"), time.Now().Add(time.Hour))
	_ = store.Create(context.Background(), "u1", hashToken("phone"), time.Now().Add(time.Hour))
	_ = store.Create(context.Background(), "u2", hashToken("other"), time.Now().Add(time.Hour))

	rec := postJSON(t, h.ResetPassword, "/api/auth/reset-password", map[string]string{
		"token": "reset-token", "new_password": "BrandNewPass!2026",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if store.count() != 1 {
		t.Fatalf("live tokens = %d, want 1 (only the other user's)", store.count())
	}
	for _, tok := range []string{"laptop", "phone"} {
		if _, err := store.Consume(context.Background(), hashToken(tok)); err == nil {
			t.Fatalf("%s should have been revoked", tok)
		}
	}
	if _, err := store.Consume(context.Background(), hashToken("other")); err != nil {
		t.Fatal("another user's session must survive")
	}
}
