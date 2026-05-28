package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/hack"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type HackHandler struct {
	service   *hack.Service
	hackStore *postgres.HackStore
}

func NewHackHandler(s *hack.Service, hs *postgres.HackStore) *HackHandler {
	return &HackHandler{service: s, hackStore: hs}
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

func (h *HackHandler) ListContestHacks(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "contestId")
	hacks, err := h.hackStore.GetByContest(r.Context(), contestID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": hacks})
}

func (h *HackHandler) ListHackableSubmissions(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "contestId")
	problemID := chi.URLParam(r, "problemId")
	submissions, err := h.hackStore.GetHackableSubmissions(r.Context(), contestID, problemID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": submissions})
}
