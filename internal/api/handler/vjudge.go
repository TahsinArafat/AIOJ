package handler

import (
	"encoding/json"
	"net/http"

	"github.com/tahsinarafat/aioj/internal/vjudge"
)

type VJudgeHandler struct {
	svc *vjudge.Service
}

func NewVJudgeHandler(svc *vjudge.Service) *VJudgeHandler {
	return &VJudgeHandler{svc: svc}
}

func (h *VJudgeHandler) ListBots(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{"bots": h.svc.GetBotNames()})
}

func (h *VJudgeHandler) Submit(w http.ResponseWriter, r *http.Request) {
	var req vjudge.SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.ProblemRemoteID == "" || req.SourceCode == "" || req.RemoteOJ == "" {
		http.Error(w, "problem_remote_id, source_code, remote_oj required", http.StatusBadRequest)
		return
	}
	if err := h.svc.Submit(r.Context(), req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
