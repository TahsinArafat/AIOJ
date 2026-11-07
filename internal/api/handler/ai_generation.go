package handler

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/generate"
	"github.com/tahsinarafat/aioj/internal/store"
	"net/http"
	"strconv"
)

type AIGenerationHandler struct {
	service  *generate.JobService
	apply    *generate.ApplyService
	jobs     store.AIGenerationStore
	models   store.AIModelStore
	problems store.ProblemStore
}

func NewAIGenerationHandler(s *generate.JobService, a *generate.ApplyService, j store.AIGenerationStore, m store.AIModelStore, p store.ProblemStore) *AIGenerationHandler {
	return &AIGenerationHandler{service: s, apply: a, jobs: j, models: m, problems: p}
}
func (h *AIGenerationHandler) Connections(w http.ResponseWriter, r *http.Request) {
	items, err := h.models.List(r.Context())
	if err != nil {
		http.Error(w, "failed to list connections", 500)
		return
	}
	enabled := items[:0]
	for _, m := range items {
		if m.Enabled {
			m.APIKey = ""
			enabled = append(enabled, m)
		}
	}
	respondJSON(w, 200, map[string]any{"data": enabled})
}
func (h *AIGenerationHandler) Presets(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, 200, map[string]any{"version": "problem-brief-v1", "data": generate.BriefPresets()})
}
func (h *AIGenerationHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	var req struct {
		ProblemID    *string          `json:"problem_id"`
		AIModelID    *string          `json:"ai_model_id"`
		ArtifactType string           `json:"artifact_type"`
		Brief        generate.Request `json:"brief"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	job, err := h.service.Create(r.Context(), claims.UserID, generate.CreateJobRequest{ProblemID: req.ProblemID, AIModelID: req.AIModelID, ArtifactType: req.ArtifactType, Brief: req.Brief, ActorIsAdmin: claims.Role == "admin"})
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	respondJSON(w, 201, job)
}
func (h *AIGenerationHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	j, err := h.jobs.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "failed to get job", 500)
		return
	}
	if j == nil {
		http.Error(w, "not found", 404)
		return
	}
	if claims.Role != "admin" && j.ActorID != claims.UserID {
		http.Error(w, "forbidden", 403)
		return
	}
	respondJSON(w, 200, j)
}
func (h *AIGenerationHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	ok, err := h.jobs.RequestCancel(r.Context(), chi.URLParam(r, "id"), claims.UserID)
	if err != nil {
		http.Error(w, "cancel failed", 500)
		return
	}
	if !ok {
		http.Error(w, "job cannot be cancelled", 409)
		return
	}
	respondJSON(w, 200, map[string]bool{"cancelled": true})
}
func (h *AIGenerationHandler) Discard(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	ok, err := h.jobs.MarkDiscarded(r.Context(), chi.URLParam(r, "id"), claims.UserID)
	if err != nil {
		http.Error(w, "discard failed", 500)
		return
	}
	if !ok {
		http.Error(w, "job cannot be discarded", 409)
		return
	}
	respondJSON(w, 200, map[string]bool{"discarded": true})
}
func (h *AIGenerationHandler) Apply(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	j, err := h.jobs.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil || j == nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	if claims.Role != "admin" && j.ActorID != claims.UserID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req generate.ApplyRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	result, err := h.apply.Apply(r.Context(), j, claims.UserID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	respondJSON(w, http.StatusOK, result)
}
func (h *AIGenerationHandler) Retry(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	old, err := h.jobs.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil || old == nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	if claims.Role != "admin" && old.ActorID != claims.UserID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var brief generate.Request
	if json.Unmarshal(old.NormalizedBrief, &brief) != nil {
		http.Error(w, "stored brief is invalid", http.StatusConflict)
		return
	}
	job, err := h.service.Create(r.Context(), claims.UserID, generate.CreateJobRequest{ProblemID: old.ProblemID, AIModelID: old.AIModelID, ArtifactType: old.ArtifactType, Brief: brief, ActorIsAdmin: claims.Role == "admin"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusCreated, job)
}
func (h *AIGenerationHandler) ListProblem(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	pid := chi.URLParam(r, "problemId")
	if claims.Role != "admin" && !h.problems.HasAccess(r.Context(), pid, claims.UserID, "owner", "co_author") {
		http.Error(w, "forbidden", 403)
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	items, err := h.jobs.ListByProblem(r.Context(), pid, offset, 50)
	if err != nil {
		http.Error(w, "failed to list jobs", 500)
		return
	}
	respondJSON(w, 200, map[string]any{"data": items})
}

// ListActor handles GET /api/ai/jobs — the signed-in setter's own jobs.
func (h *AIGenerationHandler) ListActor(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	items, total, err := h.jobs.ListByActor(r.Context(), claims.UserID, offset, limit)
	if err != nil {
		http.Error(w, "failed to list jobs", 500)
		return
	}
	respondJSON(w, 200, map[string]any{"data": items, "total": total})
}

// AdminList handles GET /api/admin/ai/jobs — admin view of all AI generation jobs.
func (h *AIGenerationHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	actorID := r.URL.Query().Get("actor_id")
	status := r.URL.Query().Get("status")
	items, total, err := h.jobs.ListAll(r.Context(), actorID, status, offset, limit)
	if err != nil {
		http.Error(w, "failed to list jobs", 500)
		return
	}
	respondJSON(w, 200, map[string]any{"data": items, "total": total})
}

// AdminCancel handles POST /api/admin/ai/jobs/{id}/cancel — admin force-cancel.
func (h *AIGenerationHandler) AdminCancel(w http.ResponseWriter, r *http.Request) {
	ok, err := h.jobs.AdminRequestCancel(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "cancel failed", 500)
		return
	}
	if !ok {
		http.Error(w, "job cannot be cancelled", 409)
		return
	}
	respondJSON(w, 200, map[string]bool{"cancelled": true})
}
