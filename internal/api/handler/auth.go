package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/mail"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type AuthHandler struct {
	users             store.UserStore
	refreshToks       store.RefreshTokenStore
	passwordResetToks store.PasswordResetTokenStore
	onsiteStore       store.OnsiteUserStore
	contestStore      store.ContestStore
	jwt               *auth.JWTManager
	evt               store.EmailVerificationTokenStore
	mail              mail.Sender
	mailTpl           *template.Template
	publicURL         string
	mailFrom          string
}

func NewAuthHandler(
	users store.UserStore,
	refreshToks store.RefreshTokenStore,
	passwordResetToks store.PasswordResetTokenStore,
	onsiteStore store.OnsiteUserStore,
	contestStore store.ContestStore,
	jwt *auth.JWTManager,
	evt store.EmailVerificationTokenStore,
	m mail.Sender,
	tpl *template.Template,
	publicURL string,
	mailFrom string,
) *AuthHandler {
	return &AuthHandler{
		users: users, refreshToks: refreshToks, passwordResetToks: passwordResetToks,
		onsiteStore: onsiteStore, contestStore: contestStore, jwt: jwt,
		evt: evt, mail: m, mailTpl: tpl, publicURL: publicURL, mailFrom: mailFrom,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		http.Error(w, "username, email, password required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 6 {
		http.Error(w, "password too short (min 6)", http.StatusBadRequest)
		return
	}
	existing, _ := h.users.GetByUsername(r.Context(), req.Username)
	if existing != nil {
		http.Error(w, "username taken", http.StatusConflict)
		return
	}
	existing, _ = h.users.GetByEmail(r.Context(), req.Email)
	if existing != nil {
		http.Error(w, "email taken", http.StatusConflict)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	user := &model.User{ID: uuid.New().String(), Username: req.Username, Email: req.Email, PasswordHash: hash, Role: "user"}
	if err := h.users.Create(r.Context(), user); err != nil {
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}
	h.sendVerificationEmail(r.Context(), user)
	respondJSON(w, http.StatusCreated, h.tokenResp(r.Context(), user))
}

// sendVerificationEmail issues a one-shot hashed token and emails a verify link.
// Failures are logged, not returned — registration still succeeds.
func (h *AuthHandler) sendVerificationEmail(ctx context.Context, user *model.User) {
	if h.evt == nil || h.mail == nil || h.mailTpl == nil || user.Email == "" {
		return
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		slog.Error("email verification rand failed", "err", err, "user_id", user.ID)
		return
	}
	rawHex := hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(rawHex))
	hash := hex.EncodeToString(sum[:])
	if err := h.evt.Create(ctx, uuid.New().String(), user.ID, hash, time.Now().Add(24*time.Hour)); err != nil {
		slog.Error("email verification token create failed", "err", err, "user_id", user.ID)
		return
	}
	verifyURL := strings.TrimRight(h.publicURL, "/") + "/verify-email?token=" + rawHex
	var body bytes.Buffer
	if err := h.mailTpl.ExecuteTemplate(&body, "email_verification.txt", map[string]string{
		"Username":      user.Username,
		"VerifyURL":     verifyURL,
		"ExpiryMinutes": "24",
	}); err != nil {
		slog.Error("render verification email failed", "err", err)
		return
	}
	from := h.mailFrom
	if from == "" {
		from = "noreply@aioj.com"
	}
	_ = h.mail.Send(ctx, &mail.Message{
		From: from, To: []string{user.Email},
		Subject: "Verify your AIOJ email", Body: body.String(),
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	user, err := h.users.GetByUsername(r.Context(), req.Username)
	if err == nil && user != nil && auth.CheckPassword(req.Password, user.PasswordHash) {
		respondJSON(w, http.StatusOK, h.tokenResp(r.Context(), user))
		return
	}

	onsiteUser, err := h.onsiteStore.GetByUsername(r.Context(), req.Username)
	if err != nil || onsiteUser == nil || !auth.CheckPassword(req.Password, onsiteUser.PasswordHash) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	var dbUser *model.User
	if onsiteUser.IsUsed && onsiteUser.UsedBy != nil {
		dbUser, err = h.users.GetByID(r.Context(), *onsiteUser.UsedBy)
		if err != nil || dbUser == nil {
			http.Error(w, "failed to retrieve user", http.StatusInternalServerError)
			return
		}
	} else {
		dbUser = &model.User{
			ID:           uuid.New().String(),
			Username:     onsiteUser.Username,
			Email:        onsiteUser.Username + "@onsite.aioj",
			PasswordHash: onsiteUser.PasswordHash,
			Role:         "contestant",
			IsBot:        false,
		}
		if err := h.users.Create(r.Context(), dbUser); err != nil {
			http.Error(w, "failed to create user", http.StatusInternalServerError)
			return
		}
		// Onsite accounts have synthetic emails (…@onsite.aioj) and cannot
		// receive mail; treat them as pre-verified so they can submit.
		_ = h.users.MarkEmailVerified(r.Context(), dbUser.ID)
		if err := h.onsiteStore.MarkUsed(r.Context(), onsiteUser.ID, dbUser.ID); err != nil {
			http.Error(w, "failed to update credential state", http.StatusInternalServerError)
			return
		}
		_ = h.onsiteStore.AutoRegister(r.Context(), onsiteUser.ContestID, dbUser.ID)
	}

	accessToken, err := h.jwt.GenerateAccessToken(dbUser.ID, dbUser.Username, dbUser.Role)
	if err != nil {
		http.Error(w, "failed to generate tokens", http.StatusInternalServerError)
		return
	}
	rawRefresh, hashedRefresh := h.jwt.GenerateRefreshToken()
	if err := h.refreshToks.Create(r.Context(), dbUser.ID, hashedRefresh, time.Now().Add(h.jwt.RefreshTTL())); err != nil {
		http.Error(w, "failed to save refresh token", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": rawRefresh,
		"user":          dbUser,
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	hsh := sha256.Sum256([]byte(req.RefreshToken))
	userID, err := h.refreshToks.Validate(r.Context(), hex.EncodeToString(hsh[:]))
	if err != nil {
		http.Error(w, "invalid refresh token", http.StatusUnauthorized)
		return
	}
	user, _ := h.users.GetByID(r.Context(), userID)
	if user == nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	respondJSON(w, http.StatusOK, h.tokenResp(r.Context(), user))
}

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req model.ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Email == "" {
		http.Error(w, "email required", http.StatusBadRequest)
		return
	}

	// Always return success to prevent email enumeration
	user, err := h.users.GetByEmail(r.Context(), req.Email)
	if err != nil || user == nil {
		respondJSON(w, http.StatusOK, map[string]string{"message": "If the email exists, a reset link has been sent"})
		return
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		respondJSON(w, http.StatusOK, map[string]string{"message": "If the email exists, a reset link has been sent"})
		return
	}
	rawToken := hex.EncodeToString(tokenBytes)
	tokenHash := sha256.Sum256([]byte(rawToken))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	tokenID := uuid.New().String()
	if err := h.passwordResetToks.Create(r.Context(), tokenID, user.ID, tokenHashStr, time.Now().Add(1*time.Hour)); err != nil {
		respondJSON(w, http.StatusOK, map[string]string{"message": "If the email exists, a reset link has been sent"})
		return
	}

	if h.mail != nil && h.mailTpl != nil && user.Email != "" {
		resetURL := strings.TrimRight(h.publicURL, "/") + "/reset-password?token=" + rawToken
		var body bytes.Buffer
		if err := h.mailTpl.ExecuteTemplate(&body, "password_reset.txt", map[string]string{
			"Username": user.Username,
			"ResetURL": resetURL,
			"Expiry":   "1 hour",
		}); err == nil {
			from := h.mailFrom
			if from == "" {
				from = "noreply@aioj.com"
			}
			_ = h.mail.Send(r.Context(), &mail.Message{
				From: from, To: []string{user.Email},
				Subject: "Reset your AIOJ password", Body: body.String(),
			})
		}
	}

	// Never return the raw token — enumeration-safe success only.
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "If the email exists, a reset link has been sent",
	})
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req model.ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Token == "" || req.NewPassword == "" {
		http.Error(w, "token and new_password required", http.StatusBadRequest)
		return
	}
	if len(req.NewPassword) < 6 {
		http.Error(w, "password too short (min 6)", http.StatusBadRequest)
		return
	}

	tokenHash := sha256.Sum256([]byte(req.Token))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	tok, err := h.passwordResetToks.GetByHash(r.Context(), tokenHashStr)
	if err != nil || tok == nil {
		http.Error(w, "invalid or expired token", http.StatusBadRequest)
		return
	}
	if tok.Used {
		http.Error(w, "token already used", http.StatusBadRequest)
		return
	}
	if time.Now().After(tok.ExpiresAt) {
		http.Error(w, "token expired", http.StatusBadRequest)
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := h.users.UpdatePassword(r.Context(), tok.UserID, hash); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := h.passwordResetToks.MarkUsed(r.Context(), tok.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "password reset successfully"})
}

// ResendVerification re-sends the verify link for the logged-in user.
// Enumeration-safe: always 200 with the same message.
func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := h.users.GetByID(r.Context(), claims.UserID)
	if err != nil || user == nil {
		respondJSON(w, http.StatusOK, map[string]string{
			"message": "If the account needs verification, a link has been sent",
		})
		return
	}
	if verified, _ := h.users.IsEmailVerified(r.Context(), user.ID); verified {
		respondJSON(w, http.StatusOK, map[string]string{
			"message": "If the account needs verification, a link has been sent",
		})
		return
	}
	h.sendVerificationEmail(r.Context(), user)
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "If the account needs verification, a link has been sent",
	})
}

func (h *AuthHandler) tokenResp(ctx context.Context, user *model.User) *model.AuthResponse {
	access, _ := h.jwt.GenerateAccessToken(user.ID, user.Username, user.Role)
	raw, hashed := h.jwt.GenerateRefreshToken()
	h.refreshToks.Create(ctx, user.ID, hashed, time.Now().Add(h.jwt.RefreshTTL()))
	return &model.AuthResponse{AccessToken: access, RefreshToken: raw, User: user}
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
