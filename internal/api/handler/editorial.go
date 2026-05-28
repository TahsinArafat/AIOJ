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

type EditorialHandler struct {
	store *postgres.EditorialStore
}

func NewEditorialHandler(s *postgres.EditorialStore) *EditorialHandler {
	return &EditorialHandler{store: s}
}

func (h *EditorialHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.CreateEditorialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	e := &model.Editorial{
		ProblemID:        req.ProblemID,
		UserID:           claims.UserID,
		Title:            req.Title,
		Content:          req.Content,
		SolutionCode:     req.SolutionCode,
		SolutionLanguage: req.SolutionLanguage,
		Approach:         req.Approach,
		TimeComplexity:   req.TimeComplexity,
		SpaceComplexity:  req.SpaceComplexity,
	}

	if req.ContestID != "" {
		cid := req.ContestID
		e.ContestID = &cid
	}

	if err := h.store.Create(r.Context(), e); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, e)
}

func (h *EditorialHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	e, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if e == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, e)
}

func (h *EditorialHandler) GetByProblem(w http.ResponseWriter, r *http.Request) {
	problemID := chi.URLParam(r, "problemId")
	editorials, err := h.store.GetByProblem(r.Context(), problemID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": editorials})
}

func (h *EditorialHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	items, total, _ := h.store.List(r.Context(), offset, limit)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}
