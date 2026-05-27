package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type ProblemHandler struct {
	store store.ProblemStore
}

func NewProblemHandler(s store.ProblemStore) *ProblemHandler {
	return &ProblemHandler{store: s}
}

func (h *ProblemHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	items, total, err := h.store.List(r.Context(), offset, limit)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":   items,
		"total":  total,
		"offset": offset,
		"limit":  limit,
	})
}

func (h *ProblemHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	p, err := h.store.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, p)
}

func (h *ProblemHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req model.CreateProblemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Slug == "" || req.Title == "" || req.Description == "" {
		http.Error(w, "slug, title, description required", http.StatusBadRequest)
		return
	}
	if req.TimeLimit <= 0 {
		req.TimeLimit = 1000
	}
	if req.MemoryLimit <= 0 {
		req.MemoryLimit = 262144
	}
	if req.Difficulty == "" {
		req.Difficulty = "easy"
	}
	prob := &model.Problem{
		ID:            uuid.New().String(),
		Slug:          req.Slug,
		Title:         req.Title,
		Description:   req.Description,
		InputFormat:   req.InputFormat,
		OutputFormat:  req.OutputFormat,
		Hint:          req.Hint,
		TimeLimit:     req.TimeLimit,
		MemoryLimit:   req.MemoryLimit,
		Difficulty:    req.Difficulty,
		Tags:          req.Tags,
		SampleCases:   req.SampleCases,
		TestCaseScore: req.TestCaseScore,
		SPJ:           req.SPJ,
		SPJLanguage:   req.SPJLanguage,
		SPJSourceCode: req.SPJSourceCode,
		Source:        "local",
		CreatedBy:     claims.UserID,
	}
	if err := h.store.Create(r.Context(), prob); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusCreated, prob)
}
