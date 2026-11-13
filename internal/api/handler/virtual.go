package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
	"github.com/tahsinarafat/aioj/internal/virtual"
)

type VirtualHandler struct {
	// contestHandlerAccess carries the shared contest access policy, so this
	// handler asks the same object every other surface asks.
	contestHandlerAccess
	service      *virtual.Service
	virtualStore *postgres.VirtualStore
	// contestStore is needed to refuse starting a virtual contest on a contest
	// the caller cannot read; the handler used to pass the id straight through.
	contestStore *postgres.ContestStore
}

// WithGrants attaches the password-entitlement store and rebuilds the access
// policy so it can consult it. Returns the handler for call-site chaining.
func (h *VirtualHandler) WithGrants(gs *postgres.ContestGrantStore) *VirtualHandler {
	h.rebuildWithGrants(h.contestStore, gs)
	return h
}

func NewVirtualHandler(s *virtual.Service, vs *postgres.VirtualStore, cs *postgres.ContestStore) *VirtualHandler {
	h := &VirtualHandler{service: s, virtualStore: vs, contestStore: cs}
	h.access = newContestAccessPolicy(cs, nil)
	return h
}

func (h *VirtualHandler) Start(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ContestID       string `json:"contest_id"`
		DurationMinutes int    `json:"duration_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	// A virtual contest replays a round's problems. Starting one on a contest the
	// caller cannot read would hand them its problem set, so the contest has to be
	// readable first — same rule as every other contest read.
	contest, err := h.contestStore.GetByID(r.Context(), req.ContestID)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}
	if decision, derr := h.access.Decide(r.Context(), contest, claims, model.SurfaceParticipate); derr != nil || !decision.Allowed() {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	v, err := h.service.StartVirtualContest(r.Context(), claims.UserID, contest.ID, req.DurationMinutes)
	if err != nil {
		if err == virtual.ErrActiveVirtualExists {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "failed to start", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, v)
}

func (h *VirtualHandler) Status(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		respondJSON(w, http.StatusOK, map[string]bool{"is_active": false})
		return
	}

	v, _ := h.virtualStore.GetActiveByUser(r.Context(), claims.UserID)
	if v == nil {
		respondJSON(w, http.StatusOK, map[string]bool{"is_active": false})
		return
	}

	status := h.service.GetStatus(v, time.Now())
	respondJSON(w, http.StatusOK, status)
}

func (h *VirtualHandler) Complete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	virtualID := chi.URLParam(r, "id")
	if err := h.service.CompleteContest(r.Context(), virtualID); err != nil {
		http.Error(w, "failed to complete", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}
