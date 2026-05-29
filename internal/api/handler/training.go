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

type TrainingHandler struct {
	store    *postgres.TrainingPlanStore
	orgStore *postgres.OrganizationStore
}

func NewTrainingHandler(s *postgres.TrainingPlanStore, os *postgres.OrganizationStore) *TrainingHandler {
	return &TrainingHandler{store: s, orgStore: os}
}

func (h *TrainingHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.CreateTrainingPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, "title required", http.StatusBadRequest)
		return
	}

	if req.OrganizationID != nil {
		role, _ := h.orgStore.GetMemberRole(r.Context(), *req.OrganizationID, claims.UserID)
		if role != "owner" && role != "admin" && claims.Role != "admin" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	p := &model.TrainingPlan{
		Title:          req.Title,
		Description:    req.Description,
		OrganizationID: req.OrganizationID,
		CreatedBy:      claims.UserID,
	}

	if err := h.store.Create(r.Context(), p); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}

	for i, sec := range req.Sections {
		s := &model.TrainingPlanSection{
			PlanID:      p.ID,
			Title:       sec.Title,
			Description: sec.Description,
			SortOrder:   i,
		}
		if err := h.store.CreateSection(r.Context(), s); err != nil {
			http.Error(w, "create section failed", http.StatusInternalServerError)
			return
		}
		for j, prob := range sec.Problems {
			if err := h.store.AddProblem(r.Context(), s.ID, prob.ProblemID, j, prob.Points); err != nil {
				http.Error(w, "add problem failed", http.StatusInternalServerError)
				return
			}
		}
	}

	respondJSON(w, http.StatusCreated, p)
}

func (h *TrainingHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	id := chi.URLParam(r, "id")

	userID := ""
	if claims != nil {
		userID = claims.UserID
	}

	detail, err := h.store.GetDetail(r.Context(), id, userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if detail == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	respondJSON(w, http.StatusOK, detail)
}

func (h *TrainingHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	orgIDParam := r.URL.Query().Get("org_id")
	publicOnly := r.URL.Query().Get("public") == "true"

	var orgID *string
	if orgIDParam != "" {
		orgID = &orgIDParam
	}

	items, total, _ := h.store.List(r.Context(), offset, limit, orgID, publicOnly)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

func (h *TrainingHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	plan, err := h.store.GetByID(r.Context(), id)
	if err != nil || plan == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if plan.OrganizationID != nil {
		role, _ := h.orgStore.GetMemberRole(r.Context(), *plan.OrganizationID, claims.UserID)
		if role != "owner" && role != "admin" && claims.Role != "admin" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	} else if plan.CreatedBy != claims.UserID && claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req model.CreateTrainingPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	updated := &model.TrainingPlan{Title: req.Title, Description: req.Description}
	if err := h.store.Update(r.Context(), id, updated); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *TrainingHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	plan, err := h.store.GetByID(r.Context(), id)
	if err != nil || plan == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if plan.OrganizationID != nil {
		role, _ := h.orgStore.GetMemberRole(r.Context(), *plan.OrganizationID, claims.UserID)
		if role != "owner" && role != "admin" && claims.Role != "admin" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	} else if plan.CreatedBy != claims.UserID && claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := h.store.Delete(r.Context(), id); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *TrainingHandler) AddSection(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	planID := chi.URLParam(r, "id")
	plan, err := h.store.GetByID(r.Context(), planID)
	if err != nil || plan == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	sections, _ := h.store.GetSections(r.Context(), planID)
	sec := &model.TrainingPlanSection{
		PlanID:      planID,
		Title:       req.Title,
		Description: req.Description,
		SortOrder:   len(sections),
	}

	if err := h.store.CreateSection(r.Context(), sec); err != nil {
		http.Error(w, "create section failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, sec)
}

func (h *TrainingHandler) DeleteSection(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sectionID := chi.URLParam(r, "sectionId")
	if err := h.store.DeleteSection(r.Context(), sectionID); err != nil {
		http.Error(w, "delete section failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *TrainingHandler) AddProblem(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sectionID := chi.URLParam(r, "sectionId")
	var req struct {
		ProblemID string `json:"problem_id"`
		Points    int    `json:"points"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	problems, _ := h.store.GetProblems(r.Context(), sectionID)
	if err := h.store.AddProblem(r.Context(), sectionID, req.ProblemID, len(problems), req.Points); err != nil {
		http.Error(w, "add problem failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{"status": "added"})
}

func (h *TrainingHandler) RemoveProblem(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	problemID := chi.URLParam(r, "problemId")
	if err := h.store.RemoveProblem(r.Context(), problemID); err != nil {
		http.Error(w, "remove problem failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (h *TrainingHandler) Enroll(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	planID := chi.URLParam(r, "id")
	plan, err := h.store.GetByID(r.Context(), planID)
	if err != nil || plan == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if plan.OrganizationID != nil {
		isMember, _ := h.orgStore.IsMember(r.Context(), *plan.OrganizationID, claims.UserID)
		if !isMember && claims.Role != "admin" {
			http.Error(w, "must be org member to enroll", http.StatusForbidden)
			return
		}
	}

	if err := h.store.Enroll(r.Context(), planID, claims.UserID); err != nil {
		http.Error(w, "enroll failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "enrolled"})
}

func (h *TrainingHandler) Unenroll(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	planID := chi.URLParam(r, "id")
	if err := h.store.Unenroll(r.Context(), planID, claims.UserID); err != nil {
		http.Error(w, "unenroll failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "unenrolled"})
}

func (h *TrainingHandler) GetEnrollments(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "id")
	enrollments, err := h.store.GetEnrollments(r.Context(), planID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": enrollments})
}

func (h *TrainingHandler) GetMyProgress(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	planID := chi.URLParam(r, "id")
	progress, err := h.store.GetProgress(r.Context(), planID, claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, progress)
}
