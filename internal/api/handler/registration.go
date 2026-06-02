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
		count, _ := h.registrationStore.GetRegistrationCount(r.Context(), contestID)
		if count >= *contest.MaxParticipants {
			http.Error(w, "contest is full", http.StatusBadRequest)
			return
		}
	}

	if err := h.registrationStore.Register(r.Context(), contestID, claims.UserID); err != nil {
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

	contestID := chi.URLParam(r, "id")
	if err := h.registrationStore.Unregister(r.Context(), contestID, claims.UserID); err != nil {
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

	contestID := chi.URLParam(r, "id")
	registered, _ := h.registrationStore.IsRegistered(r.Context(), contestID, claims.UserID)
	respondJSON(w, http.StatusOK, map[string]bool{"registered": registered})
}

func (h *RegistrationHandler) ListRegistrations(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "id")
	registrations, err := h.registrationStore.GetRegistrations(r.Context(), contestID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	count, _ := h.registrationStore.GetRegistrationCount(r.Context(), contestID)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":  registrations,
		"count": count,
	})
}
