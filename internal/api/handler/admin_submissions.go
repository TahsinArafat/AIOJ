package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
	"github.com/tahsinarafat/aioj/internal/vjudge"
)

type AdminSubmissionHandler struct {
	subStore   store.SubmissionStore
	probStore  store.ProblemStore
	vjudgeSvc  *vjudge.Service
}

func NewAdminSubmissionHandler(subStore store.SubmissionStore, probStore store.ProblemStore, vjSvc *vjudge.Service) *AdminSubmissionHandler {
	return &AdminSubmissionHandler{subStore: subStore, probStore: probStore, vjudgeSvc: vjSvc}
}

func (h *AdminSubmissionHandler) ListPendingRemote(w http.ResponseWriter, r *http.Request) {
	subs, err := h.subStore.GetPendingRemoteSubmissions(r.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": subs, "total": len(subs)})
}

func (h *AdminSubmissionHandler) Rejudge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sub, err := h.subStore.GetByID(r.Context(), id)
	if err != nil || sub == nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "submission not found"})
		return
	}

	prob, err := h.probStore.GetByID(r.Context(), sub.ProblemID)
	if err != nil || prob == nil || prob.Source == "" || prob.Source == "local" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "not a remote OJ problem"})
		return
	}

	h.subStore.UpdateStatus(r.Context(), id, model.StatusPending)
	h.subStore.UpdateRemoteID(r.Context(), id, "", "")
	h.subStore.UpdateBotID(r.Context(), id, "", "")

	respondJSON(w, http.StatusOK, map[string]string{"status": "rejudging"})
}

func (h *AdminSubmissionHandler) ForceRefresh(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sub, err := h.subStore.GetByID(r.Context(), id)
	if err != nil || sub == nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "submission not found"})
		return
	}
	if sub.RemoteID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "no remote submission ID"})
		return
	}

	prob, err := h.probStore.GetByID(r.Context(), sub.ProblemID)
	if err != nil || prob == nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "problem not found"})
		return
	}

	if h.vjudgeSvc != nil {
		go h.vjudgeSvc.ForcePoll(r.Context(), prob.Source, sub.RemoteID, sub.ID)
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "refreshing"})
}
