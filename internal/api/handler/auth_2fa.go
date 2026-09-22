package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type TwoFactorHandler struct {
	Users   store.UserStore
	Secrets store.TOTPSecretStore
	Backups store.BackupCodeStore
}

func mustClaims(r *http.Request) (*auth.Claims, bool) {
	c := middleware.GetUserClaims(r)
	return c, c != nil
}

func bindJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func (h *TwoFactorHandler) Begin(w http.ResponseWriter, r *http.Request) {
	claims, ok := mustClaims(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	if err := h.Secrets.Upsert(r.Context(), claims.UserID, secret); err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	user, _ := h.Users.GetByID(r.Context(), claims.UserID)
	username := claims.UserID
	if user != nil {
		username = user.Username
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"secret": secret,
		"uri": "otpauth://totp/AIOJ:" + username +
			"?secret=" + secret + "&issuer=AIOJ&algorithm=SHA1&digits=6&period=30",
	})
}

func (h *TwoFactorHandler) Enable(w http.ResponseWriter, r *http.Request) {
	claims, ok := mustClaims(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := bindJSON(r, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	secret, err := h.Secrets.Get(r.Context(), claims.UserID)
	if err != nil || secret == nil {
		http.Error(w, "2fa not initialized", http.StatusBadRequest)
		return
	}
	if !auth.ValidateTOTPCode(secret.Secret, req.Code) {
		http.Error(w, "invalid code", http.StatusBadRequest)
		return
	}
	if err := h.Secrets.Enable(r.Context(), claims.UserID); err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	codes := generateBackupCodes()
	for _, c := range codes {
		sum := sha256.Sum256([]byte(c))
		_ = h.Backups.Create(r.Context(), uuid.NewString(), claims.UserID, hex.EncodeToString(sum[:]))
	}
	respondJSON(w, http.StatusOK, map[string]any{"backup_codes": codes})
}

func (h *TwoFactorHandler) Disable(w http.ResponseWriter, r *http.Request) {
	claims, ok := mustClaims(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := bindJSON(r, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	u, _ := h.Users.GetByID(r.Context(), claims.UserID)
	if u == nil || !auth.CheckPassword(req.Password, u.PasswordHash) {
		http.Error(w, "invalid password", http.StatusForbidden)
		return
	}
	if err := h.Secrets.Disable(r.Context(), claims.UserID); err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

func generateBackupCodes() []string {
	out := make([]string, 10)
	for i := range out {
		b := make([]byte, 5)
		_, _ = rand.Read(b)
		out[i] = hex.EncodeToString(b)
	}
	return out
}

type TwoFactorVerifyHandler struct {
	Secrets store.TOTPSecretStore
	Backups store.BackupCodeStore
	JWT     *auth.JWTManager
	Users   store.UserStore
	Refresh store.RefreshTokenStore
}

func (h *TwoFactorVerifyHandler) Verify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChallengeID string `json:"challenge_id"`
		Code        string `json:"code"`
	}
	if err := bindJSON(r, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	parsed, err := h.JWT.ParseChallengeToken(req.ChallengeID)
	if err != nil || parsed == nil {
		http.Error(w, "invalid or expired challenge", http.StatusUnauthorized)
		return
	}
	userID := parsed.UserID
	secret, _ := h.Secrets.Get(r.Context(), userID)
	if secret == nil || !secret.Enabled {
		http.Error(w, "2fa not enabled", http.StatusBadRequest)
		return
	}
	if auth.ValidateTOTPCode(secret.Secret, req.Code) {
		h.issue(w, r, userID)
		return
	}
	sum := sha256.Sum256([]byte(req.Code))
	rows, _ := h.Backups.ListActive(r.Context(), userID)
	for _, row := range rows {
		if row.Hash == hex.EncodeToString(sum[:]) {
			_ = h.Backups.Consume(r.Context(), row.ID)
			h.issue(w, r, userID)
			return
		}
	}
	http.Error(w, "invalid code", http.StatusUnauthorized)
}

func (h *TwoFactorVerifyHandler) issue(w http.ResponseWriter, r *http.Request, userID string) {
	u, _ := h.Users.GetByID(r.Context(), userID)
	if u == nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}
	access, _ := h.JWT.GenerateAccessToken(u.ID, u.Username, u.Role)
	raw, hashed := h.JWT.GenerateRefreshToken()
	_ = h.Refresh.Create(r.Context(), u.ID, hashed, time.Now().Add(h.JWT.RefreshTTL()))
	respondJSON(w, http.StatusOK, &model.AuthResponse{
		AccessToken: access, RefreshToken: raw, User: u,
	})
}
