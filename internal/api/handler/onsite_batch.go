package handler

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type OnsiteBatchHandler struct {
	contestStore store.ContestStore
	onsiteStore  store.OnsiteUserStore
	userStore    store.UserStore
	refreshToks  store.RefreshTokenStore
	jwtManager   *auth.JWTManager
}

func NewOnsiteBatchHandler(cs store.ContestStore, os store.OnsiteUserStore, us store.UserStore, rt store.RefreshTokenStore, jwtManager *auth.JWTManager) *OnsiteBatchHandler {
	return &OnsiteBatchHandler{
		contestStore: cs,
		onsiteStore:  os,
		userStore:    us,
		refreshToks:  rt,
		jwtManager:   jwtManager,
	}
}

func (h *OnsiteBatchHandler) GenerateBatch(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contestID := chi.URLParam(r, "id")

	contest, err := h.contestStore.GetByID(r.Context(), contestID)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	contentType := r.Header.Get("Content-Type")

	var teams []model.BatchUserRequest

	if contentType == "text/csv" {
		reader := csv.NewReader(r.Body)
		records, err := reader.ReadAll()
		if err != nil {
			http.Error(w, "invalid CSV", http.StatusBadRequest)
			return
		}

		for i, record := range records {
			if i == 0 {
				continue
			}
			if len(record) >= 1 {
				team := model.BatchUserRequest{
					TeamName: record[0],
				}
				if len(record) >= 2 {
					team.Institution = record[1]
				}
				teams = append(teams, team)
			}
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&teams); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
	}

	if len(teams) == 0 {
		http.Error(w, "no teams provided", http.StatusBadRequest)
		return
	}

	created, err := h.onsiteStore.CreateBatch(r.Context(), contest.ID, teams)
	if err != nil {
		http.Error(w, "failed to generate users", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"users": created,
		"count": len(created),
	})
}

func (h *OnsiteBatchHandler) ListBatch(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contestID := chi.URLParam(r, "id")

	contest, err := h.contestStore.GetByID(r.Context(), contestID)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	users, err := h.onsiteStore.ListByContest(r.Context(), contest.ID)
	if err != nil {
		http.Error(w, "failed to list users", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":  users,
		"count": len(users),
	})
}

func (h *OnsiteBatchHandler) LoginAsTeam(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	user, err := h.onsiteStore.GetByUsername(r.Context(), req.Username)
	if err != nil || user == nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	var dbUser *model.User
	if user.IsUsed && user.UsedBy != nil {
		dbUser, err = h.userStore.GetByID(r.Context(), *user.UsedBy)
		if err != nil || dbUser == nil {
			http.Error(w, "failed to retrieve user", http.StatusInternalServerError)
			return
		}
	} else {
		dbUser = &model.User{
			ID:           uuid.New().String(),
			Username:     user.Username,
			Email:        user.Username + "@onsite.aioj",
			PasswordHash: user.PasswordHash,
			Role:         "contestant",
			IsBot:        false,
		}
		if err := h.userStore.Create(r.Context(), dbUser); err != nil {
			http.Error(w, "failed to create user", http.StatusInternalServerError)
			return
		}
		if err := h.onsiteStore.MarkUsed(r.Context(), user.ID, dbUser.ID); err != nil {
			http.Error(w, "failed to update credential state", http.StatusInternalServerError)
			return
		}
		_ = h.onsiteStore.AutoRegister(r.Context(), user.ContestID, dbUser.ID)
	}

	accessToken, err := h.jwtManager.GenerateAccessToken(dbUser.ID, dbUser.Username, dbUser.Role)
	if err != nil {
		http.Error(w, "failed to generate tokens", http.StatusInternalServerError)
		return
	}
	rawRefresh, hashedRefresh := h.jwtManager.GenerateRefreshToken()
	if err := h.refreshToks.Create(r.Context(), dbUser.ID, hashedRefresh, time.Now().Add(h.jwtManager.RefreshTTL())); err != nil {
		http.Error(w, "failed to save refresh token", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": rawRefresh,
		"user":          dbUser,
	})
}

func (h *OnsiteBatchHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userID := chi.URLParam(r, "userId")

	contestID := chi.URLParam(r, "id")
	contest, err := h.contestStore.GetByID(r.Context(), contestID)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := h.onsiteStore.DeleteByID(r.Context(), userID); err != nil {
		http.Error(w, "failed to delete", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
