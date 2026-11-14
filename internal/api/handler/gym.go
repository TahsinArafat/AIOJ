package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type GymHandler struct {
	// contestHandlerAccess carries the shared contest access policy, so this
	// handler asks the same object every other surface asks.
	contestHandlerAccess
	store store.GymStore
	// contestStore is needed to answer "may this caller see the contest this gym
	// points at" — the check the handler shipped without.
	contestStore contestReadAccess
}

// WithGrants attaches the password-entitlement store and rebuilds the access
// policy so it can consult it. Returns the handler for call-site chaining.
func (h *GymHandler) WithGrants(gs contestGrantChecker) *GymHandler {
	h.rebuildWithGrants(h.contestStore, gs)
	return h
}

func NewGymHandler(s store.GymStore, cs contestReadAccess) *GymHandler {
	h := &GymHandler{store: s, contestStore: cs}
	h.access = newContestAccessPolicy(cs, nil)
	return h
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

	// A gym is a public listing for a contest, so whoever creates one must be
	// able to read that contest. Without this check any signed-in account could
	// attach a gym to a private draft — which then published that draft's title
	// and slug through the gym list. Gyms stay auto-approved (no review queue):
	// the guard is on the contest, not on the gym.
	contest, err := h.contestStore.GetByID(r.Context(), req.ContestID)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}
	if decision, derr := h.access.Decide(r.Context(), contest, claims, model.SurfaceParticipate); derr != nil || !decision.Allowed() {
		// 404 rather than 403 so gym creation is not an oracle for which private
		// contest ids exist.
		http.Error(w, "contest not found", http.StatusNotFound)
		return
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
	var viewUserID string
	if claims := middleware.GetUserClaims(r); claims != nil {
		viewUserID = claims.UserID
	}
	g, err := h.store.GetByIDForViewer(r.Context(), id, viewUserID)
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

func (h *GymHandler) IsSolved(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	solved, err := h.store.IsSolved(r.Context(), id, claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"solved": solved})
}
