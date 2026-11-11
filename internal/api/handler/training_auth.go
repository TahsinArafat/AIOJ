package handler

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * Authorization for training plans.
 *
 * Update and Delete were the only two mutations that checked ownership; the four
 * structural ones (AddSection, DeleteSection, AddProblem, RemoveProblem) had no
 * check at all, so any signed-in account could rewrite anyone's plan by id. The
 * section and problem routes carry no plan id, so those handlers have to resolve
 * the plan themselves — which is why the check was skipped rather than merely
 * duplicated.
 */

// orgRoleReader is the one organization-store method this file needs.
type orgRoleReader interface {
	GetMemberRole(ctx context.Context, orgID, userID string) (string, error)
}

// canManageTrainingPlan reports whether claims may change this plan.
//
// Organization plans belong to the org's owner/admin. A personal plan (NULL
// OrganizationID) belongs to its creator. Global admins pass either way. A store
// error degrades to "not a member", i.e. denied — failing closed is correct here
// because the alternative is letting a database hiccup grant write access.
func canManageTrainingPlan(ctx context.Context, orgs orgRoleReader, plan *model.TrainingPlan, claims *auth.Claims) bool {
	if plan == nil {
		return false
	}
	if claims == nil {
		return false
	}
	if claims.Role == "admin" {
		return true
	}
	if plan.OrganizationID != nil {
		role, err := orgs.GetMemberRole(ctx, *plan.OrganizationID, claims.UserID)
		if err != nil {
			return false
		}
		return role == "owner" || role == "admin"
	}
	return plan.CreatedBy == claims.UserID
}

// requirePlanForSection resolves a section's plan, writes the error response on
// failure, and returns nil when the caller may not manage it. Used by the routes
// that identify a section or problem row but not the plan.
func (h *TrainingHandler) requirePlanForSection(w http.ResponseWriter, r *http.Request, sectionID string) *model.TrainingPlan {
	section, err := h.store.GetSectionByID(r.Context(), sectionID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil
	}
	if section == nil {
		http.Error(w, "section not found", http.StatusNotFound)
		return nil
	}
	plan, err := h.store.GetByID(r.Context(), section.PlanID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil
	}
	if plan == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return nil
	}
	if !canManageTrainingPlan(r.Context(), h.orgStore, plan, middleware.GetUserClaims(r)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return nil
	}
	return plan
}

// requirePlanForProblemRow does the same for a training_plan_problems row.
func (h *TrainingHandler) requirePlanForProblemRow(w http.ResponseWriter, r *http.Request, rowID string) *model.TrainingPlan {
	row, err := h.store.GetProblemRowByID(r.Context(), rowID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil
	}
	if row == nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return nil
	}
	return h.requirePlanForSection(w, r, row.SectionID)
}

// planFromParam resolves the {id} plan route param and authorizes it.
func (h *TrainingHandler) planFromParam(w http.ResponseWriter, r *http.Request) *model.TrainingPlan {
	plan, err := h.store.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil
	}
	if plan == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return nil
	}
	if !canManageTrainingPlan(r.Context(), h.orgStore, plan, middleware.GetUserClaims(r)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return nil
	}
	return plan
}
