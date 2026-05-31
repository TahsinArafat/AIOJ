package handler

import (
	"encoding/csv"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type OnsiteBatchHandler struct {
	contestStore store.ContestStore
	onsiteStore  store.OnsiteUserStore
	userStore    store.UserStore
}

func NewOnsiteBatchHandler(cs store.ContestStore, os store.OnsiteUserStore, us store.UserStore) *OnsiteBatchHandler {
	return &OnsiteBatchHandler{
		contestStore: cs,
		onsiteStore:  os,
		userStore:    us,
	}
}

func (h *OnsiteBatchHandler) GenerateBatch(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contestID := chi.URLParam(r, "contestId")

	contest, err := h.contestStore.GetByID(r.Context(), contestID)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contestID, claims.UserID, "manager") {
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

	created, err := h.onsiteStore.CreateBatch(r.Context(), contestID, teams)
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

	contestID := chi.URLParam(r, "contestId")

	contest, err := h.contestStore.GetByID(r.Context(), contestID)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	if claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contestID, claims.UserID, "manager") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	users, err := h.onsiteStore.ListByContest(r.Context(), contestID)
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

	if user.IsUsed {
		http.Error(w, "credentials already used", http.StatusForbidden)
		return
	}

	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"team_name":   user.TeamName,
		"institution": user.Institution,
		"contest_id":  user.ContestID,
		"user_id":     user.ID,
	})
}
