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

type GymHandler struct {
	store *postgres.GymStore
}

func NewGymHandler(s *postgres.GymStore) *GymHandler {
	return &GymHandler{store: s}
}

func (h *GymHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.CreateGymRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.Category == "" {
		req.Category = model.GymCategoryGeneral
	}

	g := &model.GymContest{
		ContestID:        req.ContestID,
		DifficultyRating: req.DifficultyRating,
		Category:         req.Category,
		Country:          req.Country,
		Season:           req.Season,
		Description:      req.Description,
		IsPublic:         req.IsPublic,
		CreatedBy:        claims.UserID,
	}

	if err := h.store.Create(r.Context(), g); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, g)
}

func (h *GymHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	filter := model.GymFilter{
		Category: r.URL.Query().Get("category"),
		Country:  r.URL.Query().Get("country"),
		Search:   r.URL.Query().Get("search"),
	}

	if minStr := r.URL.Query().Get("min_rating"); minStr != "" {
		min, _ := strconv.Atoi(minStr)
		filter.MinRating = &min
	}
	if maxStr := r.URL.Query().Get("max_rating"); maxStr != "" {
		max, _ := strconv.Atoi(maxStr)
		filter.MaxRating = &max
	}

	items, total, _ := h.store.List(r.Context(), offset, limit, filter)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

func (h *GymHandler) GetByID(w http.ResponseWriter, r *http.Request) {
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

func (h *GymHandler) MarkSolved(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.store.MarkSolved(r.Context(), id, claims.UserID); err != nil {
		http.Error(w, "failed to mark solved", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "solved"})
}
