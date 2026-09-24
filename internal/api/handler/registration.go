package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type RegistrationHandler struct {
	registrationStore *postgres.RegistrationStore
	contestStore      *postgres.ContestStore
}

func NewRegistrationHandler(rs *postgres.RegistrationStore, cs *postgres.ContestStore) *RegistrationHandler {
	return &RegistrationHandler{
		registrationStore: rs,
		contestStore:      cs,
	}
}

func (h *RegistrationHandler) Register(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contestID := chi.URLParam(r, "id")
	contest, err := h.contestStore.GetByID(r.Context(), contestID)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	allowed, err := h.contestStore.CheckGroupRestriction(r.Context(), contest.ID, claims.UserID)
	if err != nil {
		http.Error(w, "failed to verify group restriction", http.StatusInternalServerError)
		return
	}
	if !allowed && claims.Role != "admin" {
		http.Error(w, "this contest is restricted to group members only", http.StatusForbidden)
		return
	}

	if !contest.RegistrationRequired {
		http.Error(w, "registration not required for this contest", http.StatusBadRequest)
		return
	}

	if contest.RegistrationDeadline != nil && time.Now().After(*contest.RegistrationDeadline) {
		http.Error(w, "registration deadline passed", http.StatusBadRequest)
		return
	}

	if contest.MaxParticipants != nil {
		count, _ := h.registrationStore.GetRegistrationCount(r.Context(), contest.ID)
		if count >= *contest.MaxParticipants {
			http.Error(w, "contest is full", http.StatusBadRequest)
			return
		}
	}

	if err := h.registrationStore.Register(r.Context(), contest.ID, claims.UserID); err != nil {
		http.Error(w, "registration failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "registered"})
}

func (h *RegistrationHandler) Unregister(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contest, err := h.contestStore.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}
	if err := h.registrationStore.Unregister(r.Context(), contest.ID, claims.UserID); err != nil {
		http.Error(w, "unregister failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "unregistered"})
}

func (h *RegistrationHandler) CheckRegistration(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		respondJSON(w, http.StatusOK, map[string]bool{"registered": false})
		return
	}

	contest, err := h.contestStore.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}
	registered, _ := h.registrationStore.IsRegistered(r.Context(), contest.ID, claims.UserID)
	respondJSON(w, http.StatusOK, map[string]bool{"registered": registered})
}

func (h *RegistrationHandler) ListRegistrations(w http.ResponseWriter, r *http.Request) {
	contest, err := h.contestStore.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}
	registrations, err := h.registrationStore.GetRegistrations(r.Context(), contest.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	count, _ := h.registrationStore.GetRegistrationCount(r.Context(), contest.ID)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":  registrations,
		"count": count,
	})
}
