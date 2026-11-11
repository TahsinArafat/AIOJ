package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
	"github.com/tahsinarafat/aioj/internal/virtual"
)

type VirtualHandler struct {
	service      *virtual.Service
	virtualStore *postgres.VirtualStore
	// contestStore is needed to refuse starting a virtual contest on a contest
	// the caller cannot read; the handler used to pass the id straight through.
	contestStore *postgres.ContestStore
}

func NewVirtualHandler(s *virtual.Service, vs *postgres.VirtualStore, cs *postgres.ContestStore) *VirtualHandler {
	return &VirtualHandler{service: s, virtualStore: vs, contestStore: cs}
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
	if err != nil || contest == nil || !canReadContest(r.Context(), h.contestStore, contest, claims) {
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
