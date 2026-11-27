package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type EditorialHandler struct {
	// contestHandlerAccess carries the shared contest access policy, so this
	// handler asks the same object every other surface asks.
	contestHandlerAccess
	store        store.EditorialStore
	problemStore problemReadAccess
	contestStore contestReadAccess
}

// WithGrants attaches the password-entitlement store and rebuilds the access
// policy so it can consult it. Returns the handler for call-site chaining.
func (h *EditorialHandler) WithGrants(gs contestGrantChecker) *EditorialHandler {
	h.rebuildWithGrants(h.contestStore, gs)
	return h
}

func NewEditorialHandler(s store.EditorialStore, ps store.ProblemStore, cs contestReadAccess) *EditorialHandler {
	h := &EditorialHandler{store: s, problemStore: ps, contestStore: cs}
	h.access = newContestAccessPolicy(cs, nil)
	return h
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

	// An editorial carries the solution, so attaching one to a problem the caller
	// cannot even read would let them park solution code on a stranger's draft
	// (and name it by id). The problem must be readable first.
	p, err := h.problemStore.GetByID(r.Context(), req.ProblemID)
	if err != nil || p == nil || !canReadProblem(r.Context(), h.problemStore, p, claims) {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}
	req.ProblemID = p.ID

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
		// editorialVisible() keys off the contest, so an arbitrary contest id would
		// control when this editorial becomes readable. Require a readable one.
		contest, err := h.contestStore.GetByID(r.Context(), req.ContestID)
		if err != nil || contest == nil {
			http.Error(w, "contest not found", http.StatusNotFound)
			return
		}
		if decision, derr := h.access.Decide(r.Context(), contest, claims, model.SurfaceParticipate); derr != nil || !decision.Allowed() {
			http.Error(w, "contest not found", http.StatusNotFound)
			return
		}
		cid := contest.ID
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

	// An editorial carries the solution. On a draft that is the answer key, so
	// the problem itself must be readable first — the editorialVisible() filter
	// in the store only covers contests, not draft visibility.
	p, err := h.problemStore.GetByID(r.Context(), problemID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !canReadProblem(r.Context(), h.problemStore, p, middleware.GetUserClaims(r)) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

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

// canManageEditorial reports whether the caller may rewrite or remove this
// editorial: an admin, its author, or someone who can author the problem it
// belongs to. canReadProblem is deliberately not enough — reading a problem is
// not authoring for it.
//
// The problem's creator is checked against CreatedBy, not left to HasAccess:
// HasAccess reads problem_permissions, which does not necessarily hold a row for
// the creator, so relying on it alone would lock an owner out of their own draft.
func (h *EditorialHandler) canManageEditorial(ctx context.Context, e *model.Editorial, claims *auth.Claims) bool {
	if claims.Role == "admin" || e.UserID == claims.UserID {
		return true
	}
	p, err := h.problemStore.GetByID(ctx, e.ProblemID)
	if err != nil || p == nil {
		return false
	}
	if p.CreatedBy == claims.UserID {
		return true
	}
	return h.problemStore.HasAccess(ctx, e.ProblemID, claims.UserID, "owner", "co_author")
}

// Update rewrites one editorial's prose and solution fields. Ownership of the
// row is what grants the right, so an author can fix their own editorial
// without holding a role on the problem.
func (h *EditorialHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	existing, err := h.store.GetForManage(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// A row the caller may not manage is reported as absent, so the endpoint
	// cannot be used to probe for editorial ids.
	if existing == nil || !h.canManageEditorial(r.Context(), existing, claims) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var req model.CreateEditorialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		http.Error(w, "title and content are required", http.StatusBadRequest)
		return
	}

	existing.Title = req.Title
	existing.Content = req.Content
	existing.Approach = req.Approach
	existing.SolutionCode = req.SolutionCode
	existing.SolutionLanguage = req.SolutionLanguage
	existing.TimeComplexity = req.TimeComplexity
	existing.SpaceComplexity = req.SpaceComplexity

	if err := h.store.Update(r.Context(), existing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, existing)
}

// Delete removes one editorial. Same ownership rule as Update.
func (h *EditorialHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	existing, err := h.store.GetForManage(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if existing == nil || !h.canManageEditorial(r.Context(), existing, claims) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := h.store.Delete(r.Context(), existing.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpsertSolution creates or updates the official reference solution for a
// problem, without touching any editorial prose. This is the setter's "manage
// reference solution" path — the AI Studio apply and this endpoint share the
// same official editorial row, so a solution written here is the one a future
// editorial apply updates rather than duplicating.
func (h *EditorialHandler) UpsertSolution(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	problemID := chi.URLParam(r, "problemId")
	var req model.CreateEditorialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	p, err := h.problemStore.GetByID(r.Context(), problemID)
	if err != nil || p == nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}
	// Only the owner, an admin, or a collaborator with edit rights may write the
	// answer key. canReadProblem is not enough — reading a problem is not
	// authoring for it.
	if claims.Role != "admin" && p.CreatedBy != claims.UserID &&
		!h.problemStore.HasAccess(r.Context(), p.ID, claims.UserID, "owner", "co_author") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	e := &model.Editorial{
		ProblemID:        p.ID,
		UserID:           claims.UserID,
		SolutionCode:     req.SolutionCode,
		SolutionLanguage: req.SolutionLanguage,
		IsOfficial:       true,
	}
	if err := h.store.UpsertOfficialSolution(r.Context(), e); err != nil {
		http.Error(w, "save failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, e)
}

// GetSolution returns the official reference solution for a problem, or an empty
// object when none has been written yet. Optionally authenticated so a setter on
// a draft can read the answer key back.
func (h *EditorialHandler) GetSolution(w http.ResponseWriter, r *http.Request) {
	problemID := chi.URLParam(r, "problemId")
	e, err := h.store.GetOfficialSolution(r.Context(), problemID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if e == nil {
		respondJSON(w, http.StatusOK, &model.Editorial{ProblemID: problemID})
		return
	}
	respondJSON(w, http.StatusOK, e)
}
