package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/fps"
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

	difficulty := r.URL.Query().Get("difficulty")
	search := r.URL.Query().Get("search")
	source := r.URL.Query().Get("source")
	rating := r.URL.Query().Get("rating")
	sortBy := r.URL.Query().Get("sort")
	var filterTags []string
	if tagsStr := r.URL.Query().Get("tags"); tagsStr != "" {
		filterTags = strings.Split(tagsStr, ",")
	}

	var items []model.ProblemListItem
	var total int
	var err error

	if difficulty != "" || len(filterTags) > 0 || search != "" || source != "" || rating != "" || sortBy != "" {
		items, total, err = h.store.ListWithFilter(r.Context(), offset, limit, difficulty, filterTags, search, source, rating, sortBy)
	} else {
		items, total, err = h.store.List(r.Context(), offset, limit)
	}

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

func (h *ProblemHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.store.GetAllTags(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": tags})
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
	p.SPJ = req.SPJ
	p.SPJLanguage = req.SPJLanguage
	p.SPJSourceCode = req.SPJSourceCode
	p.CheckerType = req.CheckerType
	p.FloatEpsilon = req.FloatEpsilon
	p.Visible = req.Visible
	p.Interactive = req.Interactive
	p.InteractorLanguage = req.InteractorLanguage
	p.InteractorSourceCode = req.InteractorSourceCode
	if err := h.store.Update(r.Context(), p.ID, p); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
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
	if p.TestdataPath != "" {
		if err := os.RemoveAll(p.TestdataPath); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	if err := h.store.Delete(r.Context(), p.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProblemHandler) Export(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	prob, err := h.store.GetBySlug(r.Context(), slug)
	if err != nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}

	xmlBytes, err := fps.GenerateXML([]*model.Problem{prob})
	if err != nil {
		http.Error(w, "failed to generate FPS XML: "+err.Error(), http.StatusInternalServerError)
		return
	}

	zipBuf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(zipBuf)

	xmlFile, err := zipWriter.Create("problem.xml")
	if err != nil {
		http.Error(w, "failed to create XML in ZIP: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := xmlFile.Write(xmlBytes); err != nil {
		http.Error(w, "failed to write XML to ZIP: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if prob.TestdataPath != "" {
		files, err := os.ReadDir(prob.TestdataPath)
		if err == nil {
			for _, f := range files {
				if f.IsDir() {
					continue
				}

				fPath := filepath.Join(prob.TestdataPath, f.Name())
				data, err := os.ReadFile(fPath)
				if err != nil {
					continue
				}

				zf, err := zipWriter.Create(f.Name())
				if err != nil {
					continue
				}
				if _, err := zf.Write(data); err != nil {
					continue
				}
			}
		}
	}

	if err := zipWriter.Close(); err != nil {
		http.Error(w, "failed to close ZIP writer: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+prob.Slug+".zip\"")
	w.Write(zipBuf.Bytes())
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
	if req.CheckerType == "" {
		req.CheckerType = "exact"
	}
	if req.FloatEpsilon == 0 {
		req.FloatEpsilon = 1e-6
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
		CheckerType:   req.CheckerType,
		FloatEpsilon:  req.FloatEpsilon,
		Interactive:          req.Interactive,
		InteractorLanguage:   req.InteractorLanguage,
		InteractorSourceCode: req.InteractorSourceCode,
		Source:        "local",
		Visible:       false,
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
	perms, err := h.store.GetPermissions(r.Context(), p.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
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
	if err := h.store.AddPermission(r.Context(), p.ID, req.UserID, req.Level); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
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
	if err := h.store.RemovePermission(r.Context(), p.ID, targetUserID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ProblemHandler) ListMyProblems(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	items, total, err := h.store.ListByCreatedBy(r.Context(), claims.UserID, offset, limit)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}
