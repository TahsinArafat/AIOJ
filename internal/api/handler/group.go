package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type GroupHandler struct {
	store *postgres.GroupStore
}

func NewGroupHandler(s *postgres.GroupStore) *GroupHandler {
	return &GroupHandler{store: s}
}

func (h *GroupHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}

	g := &model.Group{
		Name:        req.Name,
		Description: req.Description,
		IsPublic:    req.IsPublic,
		MaxMembers:  req.MaxMembers,
		CreatedBy:   claims.UserID,
	}

	if err := h.store.Create(r.Context(), g); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, g)
}

func (h *GroupHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	g, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if g == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	respondJSON(w, http.StatusOK, g)
}

func (h *GroupHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	items, total, _ := h.store.List(r.Context(), offset, limit)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

func (h *GroupHandler) Join(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	groupID := chi.URLParam(r, "id")
	if err := h.store.AddMember(r.Context(), groupID, claims.UserID, "member"); err != nil {
		http.Error(w, "join failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "joined"})
}

func (h *GroupHandler) Leave(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	groupID := chi.URLParam(r, "id")
	if err := h.store.RemoveMember(r.Context(), groupID, claims.UserID); err != nil {
		http.Error(w, "leave failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "left"})
}

func (h *GroupHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "id")
	members, err := h.store.GetMembers(r.Context(), groupID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"data": members})
}

func (h *GroupHandler) AddContest(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	groupID := chi.URLParam(r, "id")
	var req struct {
		ContestID string `json:"contest_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if err := h.store.AddContest(r.Context(), groupID, req.ContestID); err != nil {
		http.Error(w, "add contest failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "added"})
}
