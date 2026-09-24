package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/hack"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type HackHandler struct {
	service      *hack.Service
	hackStore    *postgres.HackStore
	contestStore interface {
		GetByID(context.Context, string) (*model.Contest, error)
	}
}

func NewHackHandler(s *hack.Service, hs *postgres.HackStore, resolvers ...interface {
	GetByID(context.Context, string) (*model.Contest, error)
}) *HackHandler {
	var cs interface {
		GetByID(context.Context, string) (*model.Contest, error)
	}
	if len(resolvers) > 0 {
		cs = resolvers[0]
	}
	return &HackHandler{service: s, hackStore: hs, contestStore: cs}
}

func (h *HackHandler) SubmitHack(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.HackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	result, err := h.service.SubmitHack(r.Context(), claims.UserID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (h *HackHandler) GetHack(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	hackObj, err := h.hackStore.GetByID(r.Context(), id)
	if err != nil || hackObj == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, hackObj)
}

func (h *HackHandler) resolveContest(w http.ResponseWriter, r *http.Request) (*model.Contest, bool) {
	if h.contestStore == nil {
		http.Error(w, "contest lookup unavailable", http.StatusInternalServerError)
		return nil, false
	}
	contest, err := h.contestStore.GetByID(r.Context(), chi.URLParam(r, "contestId"))
	if err != nil {
		respondContestLookupError(w, err)
		return nil, false
	}
	if contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return nil, false
	}
	return contest, true
}

func (h *HackHandler) ListContestHacks(w http.ResponseWriter, r *http.Request) {
	contest, ok := h.resolveContest(w, r)
	if !ok {
		return
	}
	hacks, err := h.hackStore.GetByContest(r.Context(), contest.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": hacks})
}

func (h *HackHandler) ListHackableSubmissions(w http.ResponseWriter, r *http.Request) {
	contest, ok := h.resolveContest(w, r)
	if !ok {
		return
	}
	problemID := chi.URLParam(r, "problemId")
	submissions, err := h.hackStore.GetHackableSubmissions(r.Context(), contest.ID, problemID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": submissions})
}
