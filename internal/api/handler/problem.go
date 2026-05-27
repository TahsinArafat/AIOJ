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
	// Privacy guard: private problems only visible to admin, owner, co_author, tester
	if !p.Visible {
		claims := middleware.GetUserClaims(r)
		if claims == nil || (claims.Role != "admin" && !h.store.HasAccess(r.Context(), p.ID, claims.UserID, "owner", "co_author", "tester")) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}
	respondJSON(w, http.StatusOK, p)
}

func (h *ProblemHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	slug := chi.URLParam(r, "slug")
	p, err := h.store.GetBySlug(r.Context(), slug)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), p.ID, claims.UserID, "owner", "co_author") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req model.CreateProblemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	p.Title = req.Title
	p.Description = req.Description
	p.TimeLimit = req.TimeLimit
	p.MemoryLimit = req.MemoryLimit
	p.Difficulty = req.Difficulty
	p.InputFormat = req.InputFormat
	p.OutputFormat = req.OutputFormat
	p.Hint = req.Hint
	p.SampleCases = req.SampleCases
	p.TestCaseScore = req.TestCaseScore
	p.Tags = req.Tags
	h.store.Update(r.Context(), p.ID, p)
	w.WriteHeader(http.StatusOK)
}

func (h *ProblemHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	slug := chi.URLParam(r, "slug")
	p, err := h.store.GetBySlug(r.Context(), slug)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), p.ID, claims.UserID, "owner") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	h.store.Delete(r.Context(), p.ID)
	w.WriteHeader(http.StatusNoContent)
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

func (h *ProblemHandler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	p, err := h.store.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	perms, _ := h.store.GetPermissions(r.Context(), p.ID)
	if perms == nil {
		perms = []model.ProblemPermission{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": perms})
}

func (h *ProblemHandler) AddPermission(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	p, err := h.store.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), p.ID, claims.UserID, "owner") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		UserID string `json:"user_id"`
		Level  string `json:"access_level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	h.store.AddPermission(r.Context(), p.ID, req.UserID, req.Level)
	w.WriteHeader(http.StatusOK)
}

func (h *ProblemHandler) RemovePermission(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	p, err := h.store.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), p.ID, claims.UserID, "owner") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	targetUserID := chi.URLParam(r, "userId")
	h.store.RemovePermission(r.Context(), p.ID, targetUserID)
	w.WriteHeader(http.StatusOK)
}
