package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type AuthHandler struct {
	users            store.UserStore
	refreshToks      store.RefreshTokenStore
	passwordResetToks store.PasswordResetTokenStore
	jwt              *auth.JWTManager
}

func NewAuthHandler(users store.UserStore, refreshToks store.RefreshTokenStore, passwordResetToks store.PasswordResetTokenStore, jwt *auth.JWTManager) *AuthHandler {
	return &AuthHandler{users: users, refreshToks: refreshToks, passwordResetToks: passwordResetToks, jwt: jwt}
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
	respondJSON(w, http.StatusCreated, h.tokenResp(r.Context(), user))
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	user, err := h.users.GetByUsername(r.Context(), req.Username)
	if err != nil || user == nil || !auth.CheckPassword(req.Password, user.PasswordHash) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	respondJSON(w, http.StatusOK, h.tokenResp(r.Context(), user))
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

	// Generate random token
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

	// In a real app, send email with rawToken here.
	// For now, return the token directly for development/testing.
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "If the email exists, a reset link has been sent",
		"token":   rawToken,
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
